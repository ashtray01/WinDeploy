package profiles

import (
	"bytes"
	"fmt"
	"io"

	"go.yaml.in/yaml/v3"
)

const MaxProfileBytes = 1 << 20

// Load reads exactly one bounded YAML document. Unknown/duplicate fields,
// anchors, aliases, merge keys and excessive nesting are rejected. Inheritance
// uses Extends exclusively. Parser diagnostics are not echoed: they may include
// secret input values. Callers must keep actual secrets outside profiles.
func Load(reader io.Reader) (Profile, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxProfileBytes+1))
	if err != nil {
		return Profile{}, fmt.Errorf("could not read profile")
	}
	if len(data) > MaxProfileBytes {
		return Profile{}, fmt.Errorf("profile exceeds size limit")
	}
	var node yaml.Node
	if err = yaml.Unmarshal(data, &node); err != nil {
		return Profile{}, fmt.Errorf("invalid profile YAML")
	}
	count := 0
	var check func(*yaml.Node, int) error
	check = func(n *yaml.Node, depth int) error {
		count++
		if depth > 64 || count > 50000 {
			return fmt.Errorf("profile is too complex")
		}
		if n.Kind == yaml.AliasNode || n.Anchor != "" || n.Tag == "!!merge" {
			return fmt.Errorf("YAML aliases and merge keys are not supported; use extends")
		}
		for _, child := range n.Content {
			if err := check(child, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err = check(&node, 0); err != nil {
		return Profile{}, err
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var profile Profile
	if err = decoder.Decode(&profile); err != nil {
		return Profile{}, fmt.Errorf("invalid profile YAML schema")
	}
	var extra yaml.Node
	if err = decoder.Decode(&extra); err != io.EOF {
		return Profile{}, fmt.Errorf("expected one profile document")
	}
	if err = profile.Validate(); err != nil {
		return Profile{}, err
	}
	return profile, nil
}

// Marshal exports a structurally validated profile. It does not resolve parents
// or check real package/source compatibility. Sensitive keys/userinfo are rejected.
func Marshal(profile Profile) ([]byte, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	data, err := yaml.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("could not encode profile")
	}
	if len(data) > MaxProfileBytes {
		return nil, fmt.Errorf("profile exceeds size limit")
	}
	return data, nil
}
