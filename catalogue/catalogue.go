// Package catalogue defines how a go-tangra module describes itself to the
// gateway's module catalogue (spec 035): the descriptor a module repository
// keeps (tangra-module.yaml), the entry its release publishes
// (catalogue-entry.json) and the install bundle (bundle.zip).
//
// Module releases build entries with cmd/tangra-catalogue; the gateway parses
// and checks them with the same code. Everything here is strict: unknown
// fields are refused, names and paths are bounded, bundles are validated
// before any file is used.
package catalogue

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/url"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Schema is the descriptor and entry format version.
const Schema = 1

// Limits.
const (
	MaxEntryBytes  = 256 << 10
	MaxBundleBytes = 8 << 20
	MaxBundleFiles = 200
	maxHostInputs  = 20
	maxTemplates   = 20
)

// ErrInvalid wraps every refusal of a descriptor, entry or bundle.
var ErrInvalid = errors.New("catalogue: invalid")

func invalid(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalid, fmt.Sprintf(format, a...))
}

// Descriptor is tangra-module.yaml.
type Descriptor struct {
	Schema      int               `yaml:"schema" json:"schema"`
	Module      string            `yaml:"module" json:"module"`
	DisplayName string            `yaml:"display_name" json:"display_name"`
	Category    string            `yaml:"category" json:"category"`
	Summary     string            `yaml:"summary" json:"summary"`
	Image       string            `yaml:"image" json:"image"`
	Routes      Routes            `yaml:"routes" json:"routes"`
	MinCore     map[string]string `yaml:"min_core,omitempty" json:"min_core,omitempty"`
	Bundle      BundleSpec        `yaml:"bundle" json:"bundle"`
	HostInputs  []HostInput       `yaml:"host_inputs,omitempty" json:"host_inputs,omitempty"`
	Docs        string            `yaml:"docs,omitempty" json:"docs,omitempty"`
}

// Routes is the allow-list scope the module asks for.
type Routes struct {
	Prefixes []string `yaml:"prefixes" json:"prefixes"`
	Names    []string `yaml:"names" json:"names"`
}

// BundleSpec says where the install bundle lives and which files are templates.
type BundleSpec struct {
	Dir       string   `yaml:"dir" json:"dir,omitempty"`
	Templates []string `yaml:"templates" json:"templates"`
	// TLSHosts get a server certificate from a local CA generated at join time.
	TLSHosts []string `yaml:"tls_hosts,omitempty" json:"tls_hosts,omitempty"`
}

// HostInput is a value the operator supplies for the target host.
type HostInput struct {
	Key     string `yaml:"key" json:"key"`
	Label   string `yaml:"label" json:"label"`
	Pattern string `yaml:"pattern" json:"pattern"`
	Default string `yaml:"default,omitempty" json:"default,omitempty"`
}

// Entry is catalogue-entry.json: the descriptor of one release.
type Entry struct {
	Schema      int               `json:"schema"`
	Module      string            `json:"module"`
	Version     string            `json:"version"`
	Repository  string            `json:"repository"`
	DisplayName string            `json:"display_name"`
	Category    string            `json:"category"`
	Summary     string            `json:"summary"`
	Image       string            `json:"image"`
	Routes      Routes            `json:"routes"`
	Permissions []string          `json:"permissions"`
	MinCore     map[string]string `json:"min_core,omitempty"`
	Bundle      EntryBundle       `json:"bundle"`
	HostInputs  []HostInput       `json:"host_inputs,omitempty"`
	Docs        string            `json:"docs,omitempty"`
}

// EntryBundle pins the bundle.zip of the release.
type EntryBundle struct {
	Templates []string `json:"templates"`
	TLSHosts  []string `json:"tls_hosts,omitempty"`
	SHA256    string   `json:"sha256"`
	Size      int64    `json:"size"`
}

var (
	moduleRE  = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	imageRE   = regexp.MustCompile(`^[a-z0-9.-]+(:[0-9]{1,5})?(/[a-z0-9._-]+)+$`)
	versionRE = regexp.MustCompile(`^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$`)
	keyRE     = regexp.MustCompile(`^[A-Z][A-Z0-9_]{1,63}$`)
	hostRE    = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)*$`)
	permRE    = regexp.MustCompile(`^[a-z0-9][a-z0-9_.-]{0,63}:[a-z0-9][a-z0-9_.-]{0,63}$`)
	ownerRE   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,38}$`)
	repoRE    = regexp.MustCompile(`^[A-Za-z0-9._-]{1,100}$`)
)

// reserved are the placeholders the gateway fills at join time; host inputs
// may not shadow them.
var reserved = map[string]bool{
	"TRUST_DOMAIN": true, "GATEWAY_ISSUER": true, "LCM_ENROLL_URL": true, "AUTH_GRPC": true, "GATEWAY_GRPC": true,
	"LCM_GRPC": true, "MESH_TENANT_ID": true, "MODULE_VERSION": true, "MODULE_IMAGE": true, "MODULE": true,
}

// Reserved reports whether key is a placeholder the gateway fills itself
// (core values, MODULE_*, and every GEN_* generated value).
func Reserved(key string) bool { return reserved[key] || strings.HasPrefix(key, "GEN_") }

// ParseDescriptor decodes tangra-module.yaml strictly and validates it.
func ParseDescriptor(raw []byte) (Descriptor, error) {
	var d Descriptor
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&d); err != nil {
		return Descriptor{}, invalid("tangra-module.yaml: %v", err)
	}
	return d, d.Validate()
}

// Validate checks every field of the descriptor.
func (d Descriptor) Validate() error {
	if d.Schema != Schema {
		return invalid("schema must be %d", Schema)
	}
	if err := validateCommon(d.Module, d.DisplayName, d.Category, d.Summary, d.Image, d.Routes, d.MinCore, d.HostInputs, d.Docs); err != nil {
		return err
	}
	dir := d.Bundle.Dir
	if dir == "" || !local(dir) {
		return invalid("bundle.dir must be a relative path inside the repository")
	}
	return validateBundleSpec(d.Bundle.Templates, d.Bundle.TLSHosts)
}

func validateCommon(module, display, category, summary, image string, r Routes, minCore map[string]string, inputs []HostInput, docs string) error {
	switch {
	case !moduleRE.MatchString(module):
		return invalid("module %q must match %s", module, moduleRE)
	case display == "" || len(display) > 60:
		return invalid("display_name must be 1-60 characters")
	case len(category) > 40:
		return invalid("category must be at most 40 characters")
	case len(summary) > 200:
		return invalid("summary must be at most 200 characters")
	case !imageRE.MatchString(image):
		return invalid("image %q must be a repository without tag or digest", image)
	}
	if len(r.Prefixes) == 0 || len(r.Prefixes) > 10 {
		return invalid("routes.prefixes must list 1-10 prefixes")
	}
	for _, p := range r.Prefixes {
		if !inScope(p, module) {
			return invalid("routes.prefixes: %q is outside /api/%s, /m/%s and /%s", p, module, module, module)
		}
	}
	if len(r.Names) != 1 || r.Names[0] != module {
		return invalid("routes.names must be [%s]", module)
	}
	for k, v := range minCore {
		if k != "gateway" && k != "auth" && k != "lcm" {
			return invalid("min_core: unknown component %q (gateway, auth, lcm)", k)
		}
		if !versionRE.MatchString(v) {
			return invalid("min_core.%s: %q is not X.Y.Z", k, v)
		}
	}
	if len(inputs) > maxHostInputs {
		return invalid("at most %d host_inputs", maxHostInputs)
	}
	seen := map[string]bool{}
	for _, in := range inputs {
		switch {
		case !keyRE.MatchString(in.Key):
			return invalid("host_inputs: key %q must match %s", in.Key, keyRE)
		case Reserved(in.Key):
			return invalid("host_inputs: %s is filled by the gateway", in.Key)
		case seen[in.Key]:
			return invalid("host_inputs: %s twice", in.Key)
		case in.Label == "" || len(in.Label) > 100:
			return invalid("host_inputs.%s: label must be 1-100 characters", in.Key)
		case !strings.HasPrefix(in.Pattern, "^") || !strings.HasSuffix(in.Pattern, "$"):
			return invalid("host_inputs.%s: pattern must be anchored (^...$)", in.Key)
		}
		seen[in.Key] = true
		re, err := regexp.Compile(in.Pattern)
		if err != nil {
			return invalid("host_inputs.%s: pattern: %v", in.Key, err)
		}
		if in.Default != "" && !re.MatchString(in.Default) {
			return invalid("host_inputs.%s: default does not match the pattern", in.Key)
		}
	}
	if docs != "" {
		u, err := url.Parse(docs)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return invalid("docs must be an https URL")
		}
	}
	return nil
}

func validateBundleSpec(templates, tlsHosts []string) error {
	if len(templates) == 0 || len(templates) > maxTemplates {
		return invalid("bundle.templates must list 1-%d files", maxTemplates)
	}
	for _, t := range templates {
		if !local(t) {
			return invalid("bundle.templates: %q must be a relative path inside the bundle", t)
		}
	}
	for _, h := range tlsHosts {
		if !hostRE.MatchString(h) {
			return invalid("bundle.tls_hosts: %q is not a host name", h)
		}
	}
	return nil
}

// inScope: p is /api/<m>, /m/<m> or /<m>, or a path under one of them.
func inScope(p, module string) bool {
	if p != path.Clean(p) {
		return false
	}
	for _, root := range []string{"/api/" + module, "/m/" + module, "/" + module} {
		if p == root || strings.HasPrefix(p, root+"/") {
			return true
		}
	}
	return false
}

// local: a clean relative path that stays inside its root.
func local(p string) bool {
	return p != "" && !strings.Contains(p, `\`) && fs.ValidPath(p) && p != "."
}

// ValidVersion reports whether v is a release version X.Y.Z (no "v", no
// pre-release).
func ValidVersion(v string) bool { return versionRE.MatchString(v) }

// ValidRepository reports whether r is a GitHub owner/repo name.
func ValidRepository(r string) bool {
	owner, repo, ok := strings.Cut(r, "/")
	return ok && ownerRE.MatchString(owner) && repoRE.MatchString(repo) && repo != "." && repo != ".."
}

// ValidOwner reports whether o is a GitHub owner name.
func ValidOwner(o string) bool { return ownerRE.MatchString(o) }

// CompareVersions compares two X.Y.Z versions (-1, 0, 1).
func CompareVersions(a, b string) (int, error) {
	pa, err := parts(a)
	if err != nil {
		return 0, err
	}
	pb, err := parts(b)
	if err != nil {
		return 0, err
	}
	for i := range pa {
		switch {
		case pa[i] < pb[i]:
			return -1, nil
		case pa[i] > pb[i]:
			return 1, nil
		}
	}
	return 0, nil
}

func parts(v string) ([3]int, error) {
	var out [3]int
	if !versionRE.MatchString(v) {
		return out, invalid("version %q is not X.Y.Z", v)
	}
	for i, s := range strings.Split(v, ".") {
		out[i], _ = strconv.Atoi(s)
	}
	return out, nil
}

// BuildEntry makes the release entry for descriptor d at version.
func BuildEntry(d Descriptor, version, repository string, permissions []string, bundle []byte) (Entry, error) {
	if err := d.Validate(); err != nil {
		return Entry{}, err
	}
	if !ValidVersion(version) {
		return Entry{}, invalid("version %q is not X.Y.Z", version)
	}
	if !ValidRepository(repository) {
		return Entry{}, invalid("repository %q is not owner/repo", repository)
	}
	perms := append([]string{}, permissions...)
	for _, p := range perms {
		if !permRE.MatchString(p) {
			return Entry{}, invalid("permission %q is not resource:action", p)
		}
	}
	sort.Strings(perms)
	if _, err := ReadBundle(bundle); err != nil {
		return Entry{}, err
	}
	sum := sha256.Sum256(bundle)
	e := Entry{Schema: Schema, Module: d.Module, Version: version, Repository: repository, DisplayName: d.DisplayName, Category: d.Category,
		Summary: d.Summary, Image: d.Image, Routes: d.Routes, Permissions: perms, MinCore: d.MinCore, HostInputs: d.HostInputs, Docs: d.Docs,
		Bundle: EntryBundle{Templates: d.Bundle.Templates, TLSHosts: d.Bundle.TLSHosts, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(bundle))}}
	return e, e.Validate()
}

// ParseEntry decodes catalogue-entry.json strictly and validates it.
func ParseEntry(raw []byte) (Entry, error) {
	if len(raw) > MaxEntryBytes {
		return Entry{}, invalid("entry larger than %d bytes", MaxEntryBytes)
	}
	var e Entry
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return Entry{}, invalid("catalogue-entry.json: %v", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return Entry{}, invalid("catalogue-entry.json: trailing data")
	}
	return e, e.Validate()
}

// Validate checks every field of the entry.
func (e Entry) Validate() error {
	if e.Schema != Schema {
		return invalid("schema must be %d", Schema)
	}
	if err := validateCommon(e.Module, e.DisplayName, e.Category, e.Summary, e.Image, e.Routes, e.MinCore, e.HostInputs, e.Docs); err != nil {
		return err
	}
	switch {
	case !ValidVersion(e.Version):
		return invalid("version %q is not X.Y.Z", e.Version)
	case !ValidRepository(e.Repository):
		return invalid("repository %q is not owner/repo", e.Repository)
	case len(e.Bundle.SHA256) != 64:
		return invalid("bundle.sha256 missing")
	case e.Bundle.Size <= 0 || e.Bundle.Size > MaxBundleBytes:
		return invalid("bundle.size out of range")
	}
	for _, p := range e.Permissions {
		if !permRE.MatchString(p) {
			return invalid("permission %q is not resource:action", p)
		}
	}
	return validateBundleSpec(e.Bundle.Templates, e.Bundle.TLSHosts)
}

// Marshal encodes the entry as indented JSON.
func (e Entry) Marshal() ([]byte, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// CheckBundle verifies that bundle is the one the entry pins and is a valid
// bundle whose templates exist.
func (e Entry) CheckBundle(bundle []byte) error {
	sum := sha256.Sum256(bundle)
	if hex.EncodeToString(sum[:]) != e.Bundle.SHA256 || int64(len(bundle)) != e.Bundle.Size {
		return invalid("bundle does not match the entry's digest")
	}
	files, err := ReadBundle(bundle)
	if err != nil {
		return err
	}
	for _, t := range e.Bundle.Templates {
		if _, ok := files[t]; !ok {
			return invalid("bundle lacks template %s", t)
		}
	}
	return nil
}

// zipTime is the fixed modification time of every packed file, so the same
// tree always packs to the same bytes (and digest).
var zipTime = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// PackBundle zips every regular file of fsys deterministically.
func PackBundle(fsys fs.FS) ([]byte, error) {
	var names []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if !d.Type().IsRegular() {
			return invalid("bundle: %s is not a regular file", p)
		}
		names = append(names, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	if len(names) == 0 || len(names) > MaxBundleFiles {
		return nil, invalid("bundle must hold 1-%d files", MaxBundleFiles)
	}
	sort.Strings(names)
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, n := range names {
		data, err := fs.ReadFile(fsys, n)
		if err != nil {
			return nil, err
		}
		h := &zip.FileHeader{Name: n, Method: zip.Deflate, Modified: zipTime}
		h.SetMode(0o644)
		f, err := w.CreateHeader(h)
		if err != nil {
			return nil, err
		}
		if _, err := f.Write(data); err != nil {
			return nil, err
		}
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	if buf.Len() > MaxBundleBytes {
		return nil, invalid("bundle larger than %d bytes", MaxBundleBytes)
	}
	return buf.Bytes(), nil
}

// ReadBundle validates a bundle zip and returns its files by path. It
// refuses archives over the size or file limits, paths that escape the root,
// absolute or backslashed paths, and anything that is not a regular file.
func ReadBundle(data []byte) (map[string][]byte, error) {
	if len(data) > MaxBundleBytes {
		return nil, invalid("bundle larger than %d bytes", MaxBundleBytes)
	}
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, invalid("bundle is not a zip archive")
	}
	if len(r.File) == 0 || len(r.File) > MaxBundleFiles {
		return nil, invalid("bundle must hold 1-%d files", MaxBundleFiles)
	}
	out := make(map[string][]byte, len(r.File))
	var total int64
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "/") && f.Mode().IsDir() {
			continue
		}
		if !local(f.Name) {
			return nil, invalid("bundle: unsafe path %q", f.Name)
		}
		if !f.Mode().IsRegular() {
			return nil, invalid("bundle: %s is not a regular file", f.Name)
		}
		if _, dup := out[f.Name]; dup {
			return nil, invalid("bundle: %s twice", f.Name)
		}
		total += int64(f.UncompressedSize64)
		if f.UncompressedSize64 > MaxBundleBytes || total > 4*MaxBundleBytes {
			return nil, invalid("bundle expands beyond the limit")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, invalid("bundle: %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(io.LimitReader(rc, MaxBundleBytes+1))
		_ = rc.Close()
		if err != nil || int64(len(b)) > MaxBundleBytes {
			return nil, invalid("bundle: %s unreadable or too large", f.Name)
		}
		out[f.Name] = b
	}
	return out, nil
}
