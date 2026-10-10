package catalogue

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"testing/fstest"
)

const goodYAML = `schema: 1
module: sms-gw
display_name: SMS Gateway
category: Communications
summary: Hermes SMS API, carrier receipts, callbacks and tenant management.
image: ghcr.io/go-tangra/go-tangra-sms-gw
routes:
  prefixes: [/api/sms-gw, /m/sms-gw, /sms-gw]
  names: [sms-gw]
min_core: { gateway: 4.9.0, auth: 4.10.0 }
bundle:
  dir: deploy/bundle
  templates: [compose.yaml, config.yaml]
  tls_hosts: [local-sms-gw-db]
host_inputs:
  - key: MODULE_ADVERTISE_HOST
    label: Host name other modules use to reach it
    pattern: '^[a-z0-9][a-z0-9.-]{0,252}$'
  - key: MODULE_BIND_IP
    label: Private IP its mesh ports bind to
    pattern: '^(\d{1,3}\.){3}\d{1,3}$'
    default: 10.0.0.5
docs: https://github.com/go-tangra/go-tangra-sms-gw#readme
`

func TestParseDescriptor(t *testing.T) {
	d, err := ParseDescriptor([]byte(goodYAML))
	if err != nil {
		t.Fatal(err)
	}
	if d.Module != "sms-gw" || len(d.HostInputs) != 2 || d.Bundle.Dir != "deploy/bundle" || d.MinCore["gateway"] != "4.9.0" {
		t.Fatalf("%+v", d)
	}
	mut := func(from, to string) string { return strings.Replace(goodYAML, from, to, 1) }
	for name, doc := range map[string]string{
		"unknown key":           goodYAML + "extra: 1\n",
		"schema 2":              mut("schema: 1", "schema: 2"),
		"bad module":            mut("module: sms-gw", "module: SMS_gw"),
		"missing display name":  mut("display_name: SMS Gateway\n", ""),
		"long summary":          mut("summary: Hermes", "summary: "+strings.Repeat("x", 201)+" Hermes"),
		"image with tag":        mut("go-tangra-sms-gw\n", "go-tangra-sms-gw:4.2.0\n"),
		"foreign prefix":        mut("[/api/sms-gw, /m/sms-gw, /sms-gw]", "[/api/sms-gw, /api/auth]"),
		"prefix look-alike":     mut("[/api/sms-gw, /m/sms-gw, /sms-gw]", "[/api/sms-gwx]"),
		"no prefixes":           mut("[/api/sms-gw, /m/sms-gw, /sms-gw]", "[]"),
		"other name":            mut("names: [sms-gw]", "names: [sms-gw, auth]"),
		"bad min_core key":      mut("gateway: 4.9.0", "portal: 4.9.0"),
		"bad min_core version":  mut("gateway: 4.9.0", "gateway: latest"),
		"bundle dir escapes":    mut("dir: deploy/bundle", "dir: ../secrets"),
		"absolute template":     mut("[compose.yaml, config.yaml]", "[/etc/passwd]"),
		"template escapes":      mut("[compose.yaml, config.yaml]", "[../x.yaml]"),
		"bad tls host":          mut("[local-sms-gw-db]", "[bad_host]"),
		"bad input key":         mut("key: MODULE_BIND_IP", "key: bind-ip"),
		"reserved input key":    mut("key: MODULE_BIND_IP", "key: GATEWAY_ISSUER"),
		"generated input key":   mut("key: MODULE_BIND_IP", "key: GEN_PASSWORD_1"),
		"unanchored pattern":    mut(`pattern: '^(\d{1,3}\.){3}\d{1,3}$'`, `pattern: '\d+'`),
		"bad pattern":           mut(`pattern: '^(\d{1,3}\.){3}\d{1,3}$'`, `pattern: '^($'`),
		"default fails pattern": mut("default: 10.0.0.5", "default: nope"),
		"duplicate input":       mut("key: MODULE_BIND_IP", "key: MODULE_ADVERTISE_HOST"),
		"http docs":             mut("docs: https://", "docs: http://"),
		"not yaml":              "schema: [",
	} {
		if _, err := ParseDescriptor([]byte(doc)); err == nil {
			t.Errorf("%s accepted", name)
		} else if !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v is not ErrInvalid", name, err)
		}
	}
}

func bundleFS() fstest.MapFS {
	return fstest.MapFS{
		"compose.yaml":   {Data: []byte("services:\n  sms-gw:\n    image: ${MODULE_IMAGE}:${MODULE_VERSION}\n"), Mode: 0o644},
		"config.yaml":    {Data: []byte("trust_domain: ${TRUST_DOMAIN}\n"), Mode: 0o644},
		"docs/README.md": {Data: []byte("# sms-gw\n"), Mode: 0o644},
	}
}

func TestPackBundleIsDeterministicAndReadable(t *testing.T) {
	a, err := PackBundle(bundleFS())
	if err != nil {
		t.Fatal(err)
	}
	b, _ := PackBundle(bundleFS())
	if !bytes.Equal(a, b) {
		t.Fatal("packing the same tree twice gave different bytes (the digest must be stable)")
	}
	files, err := ReadBundle(a)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 3 || string(files["config.yaml"]) != "trust_domain: ${TRUST_DOMAIN}\n" || files["docs/README.md"] == nil {
		t.Fatalf("%v", files)
	}
}

func zipOf(t *testing.T, add func(w *zip.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	add(w)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestReadBundleRefusesHostileArchives(t *testing.T) {
	file := func(name string) func(w *zip.Writer) {
		return func(w *zip.Writer) { f, _ := w.Create(name); _, _ = f.Write([]byte("x")) }
	}
	symlink := func(w *zip.Writer) {
		h := &zip.FileHeader{Name: "link"}
		h.SetMode(0o777 | 1<<27) // os.ModeSymlink
		f, _ := w.CreateHeader(h)
		_, _ = f.Write([]byte("/etc/passwd"))
	}
	many := func(w *zip.Writer) {
		for i := 0; i <= MaxBundleFiles; i++ {
			f, _ := w.Create(strings.Repeat("a", 1) + string(rune('a'+i%26)) + "/" + strings.Repeat("f", 1+i/26) + ".txt")
			_, _ = f.Write([]byte("x"))
		}
	}
	for name, z := range map[string][]byte{
		"traversal":   zipOf(t, file("../evil")),
		"nested trav": zipOf(t, file("a/../../evil")),
		"absolute":    zipOf(t, file("/etc/cron.d/x")),
		"backslash":   zipOf(t, file(`a\..\..\evil`)),
		"symlink":     zipOf(t, symlink),
		"too many":    zipOf(t, many),
		"not a zip":   []byte("PK nope"),
	} {
		if _, err := ReadBundle(z); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if _, err := ReadBundle(make([]byte, MaxBundleBytes+1)); err == nil {
		t.Error("oversized accepted")
	}
}

func TestBuildAndParseEntry(t *testing.T) {
	d, _ := ParseDescriptor([]byte(goodYAML))
	zipped, _ := PackBundle(bundleFS())
	e, err := BuildEntry(d, "4.3.0", "go-tangra/go-tangra-sms-gw", []string{"providers:read", "messages:send"}, zipped)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(zipped)
	if e.Version != "4.3.0" || e.Bundle.SHA256 != hex.EncodeToString(sum[:]) || e.Bundle.Size != int64(len(zipped)) || len(e.Permissions) != 2 || e.Repository != "go-tangra/go-tangra-sms-gw" {
		t.Fatalf("%+v", e)
	}
	raw, err := e.Marshal()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseEntry(raw)
	if err != nil || back.Module != "sms-gw" || back.Bundle.SHA256 != e.Bundle.SHA256 {
		t.Fatalf("%+v %v", back, err)
	}
	// The entry checks the bundle it names.
	if err := back.CheckBundle(zipped); err != nil {
		t.Fatal(err)
	}
	if err := back.CheckBundle(append(zipped, 0)); err == nil {
		t.Fatal("tampered bundle accepted")
	}
	for name, v := range map[string]string{"prerelease": "4.3.0-rc.1", "v prefix": "v4.3.0", "two parts": "4.3"} {
		if _, err := BuildEntry(d, v, "go-tangra/go-tangra-sms-gw", nil, zipped); err == nil {
			t.Errorf("%s version accepted", name)
		}
	}
	if _, err := BuildEntry(d, "4.3.0", "https://github.com/x/y", nil, zipped); err == nil {
		t.Error("URL as repository accepted")
	}
	if _, err := BuildEntry(d, "4.3.0", "go-tangra/go-tangra-sms-gw", []string{"Bad Perm"}, zipped); err == nil {
		t.Error("bad permission accepted")
	}
	if _, err := ParseEntry(append(raw[:len(raw)-1], []byte(`,"x":1}`)...)); err == nil {
		t.Error("unknown entry field accepted")
	}
	if _, err := ParseEntry(make([]byte, MaxEntryBytes+1)); err == nil {
		t.Error("oversized entry accepted")
	}
}

func TestCompareVersions(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		want int
	}{{"4.10.0", "4.9.9", 1}, {"4.2.0", "4.2.0", 0}, {"4.2.0", "4.2.1", -1}, {"10.0.0", "9.99.99", 1}} {
		if got, err := CompareVersions(tc.a, tc.b); err != nil || got != tc.want {
			t.Errorf("%s vs %s: %d %v", tc.a, tc.b, got, err)
		}
	}
	if _, err := CompareVersions("4.2", "4.2.0"); err == nil {
		t.Error("bad version compared")
	}
}

func TestReservedPlaceholders(t *testing.T) {
	for _, k := range []string{"TRUST_DOMAIN", "GATEWAY_ISSUER", "LCM_ENROLL_URL", "AUTH_GRPC", "GATEWAY_GRPC", "LCM_GRPC", "MESH_TENANT_ID", "MODULE_VERSION", "MODULE_IMAGE"} {
		if !Reserved(k) {
			t.Errorf("%s not reserved", k)
		}
	}
	if Reserved("MODULE_BIND_IP") {
		t.Error("host input key reserved")
	}
}
