// Command render turns tools/split/services.yaml + tools/split/templates/*.tmpl
// into the per-repository files of the go-tangra v4 split (standalone Dockerfile,
// .dockerignore, GitHub Actions CI) and answers manifest lookups for the shell
// scripts (for example the spec directories extract.sh must keep).
//
// Templates use [[ ]] delimiters so GitHub Actions ${{ }} and metadata-action
// {{version}} placeholders pass through untouched.
//
//	go run ./tools/split/cmd/render -service ipam -out /tmp/ipam          # Dockerfile, .dockerignore, .github/workflows/ci.yaml
//	go run ./tools/split/cmd/render -platform -out /tmp/platform          # .github/workflows/ci.yaml for go-tangra/go-tangra
//	go run ./tools/split/cmd/render -service auth -print specs            # one value per line
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

type manifest struct {
	Platform platform  `yaml:"platform"`
	Services []service `yaml:"services"`
}

type platform struct {
	Repo          string   `yaml:"repo"`
	NewRepo       bool     `yaml:"new_repo"`
	DefaultBranch string   `yaml:"default_branch"`
	Level         string   `yaml:"level"`
	Module        string   `yaml:"module"`
	Contrib       []string `yaml:"contrib"`
	Specs         []string `yaml:"specs"`
	NPMPackage    string   `yaml:"npm_package"`
	NPMWorkspace  string   `yaml:"npm_workspace"`
}

type service struct {
	Name          string   `yaml:"name"`
	Dir           string   `yaml:"dir"`
	Repo          string   `yaml:"repo"`
	NewRepo       bool     `yaml:"new_repo"`
	DefaultBranch string   `yaml:"default_branch"`
	Level         string   `yaml:"level"`
	Module        string   `yaml:"module"`
	SDK           bool     `yaml:"sdk"`
	Specs         []string `yaml:"specs"`
	UI            ui       `yaml:"ui"`
	Binaries      []binary `yaml:"binaries"`
	VersionVar    bool     `yaml:"version_var"`
	Buf           bool     `yaml:"buf"`
	Runtime       runtime  `yaml:"runtime"`
}

type ui struct {
	Dir          string   `yaml:"dir"`
	BuildScripts []string `yaml:"build_scripts"`
	Dist         []string `yaml:"dist"`
}

type binary struct {
	Name string `yaml:"name"`
	Cmd  string `yaml:"cmd"`
	Tags string `yaml:"tags"`
}

type runtime struct {
	APK        []string `yaml:"apk"`
	Setcap     []string `yaml:"setcap"`
	Expose     []string `yaml:"expose"`
	Entrypoint []string `yaml:"entrypoint"`
	Cmd        []string `yaml:"cmd"`
}

// HasUI reports whether the service ships a front-end.
func (s service) HasUI() bool { return s.UI.Dir != "" && s.UI.Dir != "none" }

// RepoName is the repository name without the owner.
func (s service) RepoName() string { return s.Repo[strings.LastIndex(s.Repo, "/")+1:] }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "render:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		manifestPath = flag.String("manifest", "", "path to services.yaml (default: next to this tool, tools/split/services.yaml)")
		templatesDir = flag.String("templates", "", "templates directory (default: <manifest dir>/templates)")
		name         = flag.String("service", "", "service name from services.yaml")
		plat         = flag.Bool("platform", false, "render the platform repository (go-tangra/go-tangra) instead of a service")
		out          = flag.String("out", "", "output directory (repository root); files are written below it")
		what         = flag.String("what", "all", "service: all|dockerfile|dockerignore|ci; platform: all|ci")
		printField   = flag.String("print", "", "print a manifest field instead of rendering: specs|repo|dir|default_branch|level|sdk|new_repo|module|names")
	)
	flag.Parse()

	if *manifestPath == "" {
		*manifestPath = filepath.Join("tools", "split", "services.yaml")
	}
	if *templatesDir == "" {
		*templatesDir = filepath.Join(filepath.Dir(*manifestPath), "templates")
	}
	raw, err := os.ReadFile(*manifestPath)
	if err != nil {
		return err
	}
	var m manifest
	dec := yaml.NewDecoder(strings.NewReader(string(raw)))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return fmt.Errorf("parse %s: %w", *manifestPath, err)
	}

	if *printField == "names" {
		for _, s := range m.Services {
			fmt.Println(s.Name)
		}
		return nil
	}

	if *plat {
		if *printField != "" {
			return printPlatform(m.Platform, *printField)
		}
		if *out == "" {
			return errors.New("-out is required")
		}
		if *what != "all" && *what != "ci" {
			return fmt.Errorf("platform supports -what all|ci, got %q", *what)
		}
		return renderFile(*templatesDir, "platform-ci.yaml.tmpl", filepath.Join(*out, ".github", "workflows", "ci.yaml"), m.Platform)
	}

	if *name == "" {
		return errors.New("-service or -platform is required")
	}
	svc, ok := find(m.Services, *name)
	if !ok {
		return fmt.Errorf("unknown service %q (see -print names)", *name)
	}
	if err := validate(svc); err != nil {
		return err
	}
	if *printField != "" {
		return printService(svc, *printField)
	}
	if *out == "" {
		return errors.New("-out is required")
	}

	targets := map[string][2]string{
		"dockerfile":   {"Dockerfile.tmpl", "Dockerfile"},
		"dockerignore": {"dockerignore.tmpl", ".dockerignore"},
		"ci":           {"ci.yaml.tmpl", filepath.Join(".github", "workflows", "ci.yaml")},
	}
	order := []string{"dockerfile", "dockerignore", "ci"}
	if *what != "all" {
		if _, ok := targets[*what]; !ok {
			return fmt.Errorf("unknown -what %q", *what)
		}
		order = []string{*what}
	}
	for _, k := range order {
		t := targets[k]
		if err := renderFile(*templatesDir, t[0], filepath.Join(*out, t[1]), svc); err != nil {
			return err
		}
	}
	return nil
}

func find(svcs []service, name string) (service, bool) {
	for _, s := range svcs {
		if s.Name == name {
			return s, true
		}
	}
	return service{}, false
}

func validate(s service) error {
	switch {
	case s.Repo == "" || !strings.Contains(s.Repo, "/"):
		return fmt.Errorf("%s: repo must be owner/name", s.Name)
	case s.DefaultBranch == "":
		return fmt.Errorf("%s: default_branch is required", s.Name)
	case len(s.Binaries) == 0:
		return fmt.Errorf("%s: at least one binary is required", s.Name)
	case len(s.Runtime.Entrypoint) == 0:
		return fmt.Errorf("%s: runtime.entrypoint is required", s.Name)
	case s.HasUI() && len(s.UI.BuildScripts) == 0:
		return fmt.Errorf("%s: ui.build_scripts is required when ui.dir is set", s.Name)
	}
	return nil
}

func printService(s service, field string) error {
	switch field {
	case "specs":
		for _, d := range s.Specs {
			fmt.Println(d)
		}
	case "repo":
		fmt.Println(s.Repo)
	case "dir":
		fmt.Println(s.Dir)
	case "default_branch":
		fmt.Println(s.DefaultBranch)
	case "level":
		fmt.Println(s.Level)
	case "sdk":
		fmt.Println(s.SDK)
	case "new_repo":
		fmt.Println(s.NewRepo)
	case "module":
		fmt.Println(s.Module)
	default:
		return fmt.Errorf("unknown -print field %q", field)
	}
	return nil
}

func printPlatform(p platform, field string) error {
	switch field {
	case "specs":
		for _, d := range p.Specs {
			fmt.Println(d)
		}
	case "repo":
		fmt.Println(p.Repo)
	case "default_branch":
		fmt.Println(p.DefaultBranch)
	case "module":
		fmt.Println(p.Module)
	default:
		return fmt.Errorf("unknown -print field %q for platform", field)
	}
	return nil
}

var funcs = template.FuncMap{
	"join": strings.Join,
	// jsonArray renders a Dockerfile exec-form array.
	"jsonArray": func(v []string) (string, error) {
		b, err := json.Marshal(v)
		return string(b), err
	},
	"binNames": func(bs []binary) string {
		out := make([]string, 0, len(bs))
		for _, b := range bs {
			out = append(out, "/out/"+b.Name)
		}
		return strings.Join(out, " ")
	},
}

func renderFile(dir, tmplName, dest string, data any) error {
	t, err := template.New(tmplName).Delims("[[", "]]").Funcs(funcs).Option("missingkey=error").
		ParseFiles(filepath.Join(dir, tmplName))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if err := t.Execute(f, data); err != nil {
		f.Close()
		return fmt.Errorf("%s: %w", tmplName, err)
	}
	if err := f.Close(); err != nil {
		return err
	}
	fmt.Println("wrote", dest)
	return nil
}
