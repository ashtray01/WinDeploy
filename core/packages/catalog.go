package packages

import (
	"bytes"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

// MaxCatalogBytes bounds catalog input. The catalog contains no secrets, but a
// bounded read keeps a malformed file from exhausting memory.
const MaxCatalogBytes = 1 << 20

// LoadCatalog reads exactly one bounded YAML document with strict field checking.
// Unknown/duplicate fields, anchors, aliases and merge keys are rejected.
// Validation is structural only: source URLs and hashes are checked, but no
// network access or target-system probing happens here.
func LoadCatalog(reader io.Reader) (Catalog, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxCatalogBytes+1))
	if err != nil {
		return Catalog{}, fmt.Errorf("could not read package catalog")
	}
	if len(data) > MaxCatalogBytes {
		return Catalog{}, fmt.Errorf("package catalog exceeds size limit")
	}
	var node yaml.Node
	if err = yaml.Unmarshal(data, &node); err != nil {
		return Catalog{}, fmt.Errorf("invalid package catalog YAML")
	}
	count := 0
	var check func(*yaml.Node, int) error
	check = func(n *yaml.Node, depth int) error {
		count++
		if depth > 64 || count > 200000 {
			return fmt.Errorf("package catalog is too complex")
		}
		if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag == "!!merge" {
			return fmt.Errorf("YAML aliases and merge keys are not supported")
		}
		for _, child := range n.Content {
			if err := check(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err = check(&node, 0); err != nil {
		return Catalog{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var catalog Catalog
	if err = decoder.Decode(&catalog); err != nil {
		return Catalog{}, fmt.Errorf("invalid package catalog schema")
	}
	var extra yaml.Node
	if err = decoder.Decode(&extra); err != io.EOF {
		return Catalog{}, fmt.Errorf("expected one package catalog document")
	}
	if err = catalog.Validate(); err != nil {
		return Catalog{}, err
	}
	return catalog, nil
}

// Find returns a package by exact ID.
func (c Catalog) Find(id string) (Package, bool) {
	for _, p := range c.Packages {
		if p.ID == id {
			return p, true
		}
	}
	return Package{}, false
}

// ValidateSelection checks that every requested ID exists and is unique. An empty
// selection means every catalog package.
func (c Catalog) ValidateSelection(selection []string) error {
	if len(selection) == 0 {
		return nil
	}
	seen := map[string]bool{}
	for _, id := range selection {
		if seen[id] {
			return fmt.Errorf("duplicate selected package %q", id)
		}
		seen[id] = true
		if _, ok := c.Find(id); !ok {
			return fmt.Errorf("selected package %q is not in the catalog", id)
		}
	}
	return nil
}
