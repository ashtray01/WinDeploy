package profiles

import "fmt"

func copyStrings(values []string) []string {
	if values == nil {
		return nil
	}
	result := make([]string, len(values))
	copy(result, values)
	return result
}

func copyComponent(source Component) Component {
	result := source
	result.Dependencies = copyStrings(source.Dependencies)
	result.Requirements.Versions = copyStrings(source.Requirements.Versions)
	result.Requirements.Architectures = copyStrings(source.Requirements.Architectures)
	if source.Parameters != nil {
		result.Parameters = make(map[string]string, len(source.Parameters))
		for key, value := range source.Parameters {
			result.Parameters[key] = value
		}
	}
	return result
}

func copyProfile(source Profile) Profile {
	result := source
	if source.Components != nil {
		result.Components = make([]Component, len(source.Components))
		for i, c := range source.Components {
			result.Components[i] = copyComponent(c)
		}
	}
	return result
}

// Resolve flattens single-parent inheritance. Child components replace matching
// IDs completely (not field-by-field), preserving the parent's position. New IDs
// append in child order. Disabled tombstones remain in the result so plan building
// can reject dependencies on disabled components. All output data is copied.
// The registry is explicit: Extends never triggers network or filesystem access.
func Resolve(registry map[string]Profile, id string) (Profile, error) {
	visiting := map[string]bool{}
	var resolve func(string, int) (Profile, error)
	resolve = func(id string, depth int) (Profile, error) {
		if depth >= 64 {
			return Profile{}, fmt.Errorf("profile inheritance is too deep")
		}
		if visiting[id] {
			return Profile{}, fmt.Errorf("profile inheritance cycle")
		}
		profile, exists := registry[id]
		if !exists {
			return Profile{}, fmt.Errorf("profile or parent not found")
		}
		if profile.ID != id {
			return Profile{}, fmt.Errorf("registry key differs from profile ID")
		}
		if err := profile.Validate(); err != nil {
			return Profile{}, err
		}
		result := copyProfile(profile)
		if profile.Extends == "" {
			return result, nil
		}
		visiting[id] = true
		defer delete(visiting, id)
		parent, err := resolve(profile.Extends, depth+1)
		if err != nil {
			return Profile{}, err
		}
		positions := make(map[string]int, len(parent.Components))
		for i, c := range parent.Components {
			positions[c.ID] = i
		}
		merged := parent.Components
		for _, c := range result.Components {
			if index, exists := positions[c.ID]; exists {
				merged[index] = c
			} else {
				positions[c.ID] = len(merged)
				merged = append(merged, c)
			}
		}
		result.Components = merged
		result.Extends = ""
		return result, nil
	}
	return resolve(id, 0)
}
