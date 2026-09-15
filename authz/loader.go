package authz

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

// Load parses a YAML or JSON policy document (JSON is a YAML subset), rejects
// unknown fields, and validates it against the schema rules.
func Load(r io.Reader) (*Policy, error) {
	raw, err := io.ReadAll(io.LimitReader(r, 8<<20))
	if err != nil {
		return nil, fmt.Errorf("authz: read: %w", err)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil, errors.New("authz: empty policy document")
	}
	var doc struct {
		Version *string `yaml:"version"`
		Rules   *[]Rule `yaml:"rules"`
	}
	dec := yaml.NewDecoder(bytes.NewReader(raw))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, fmt.Errorf("authz: parse: %w", err)
	}
	if doc.Version == nil {
		return nil, errors.New("authz: version is required")
	}
	if doc.Rules == nil {
		return nil, errors.New("authz: rules is required (use [] for none)")
	}
	return NewPolicy(*doc.Version, *doc.Rules)
}
