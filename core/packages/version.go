package packages

import "strings"

// CompareVersions compares dotted numeric versions such as "24.08", "1.2.3.4"
// or "26.10.3.771". Leading "v" is ignored. Trailing zero components are equal
// ("1.2" == "1.2.0"). A non-numeric suffix starts a pre-release comparison:
// 1.0.0 < 1.0.0-beta < 1.0.0-rc < 1.0.0 release, and suffixes compare
// lexicographically for equal numeric parts. Unknown token shapes fall back to a
// deterministic case-insensitive string comparison after numeric parts.
func CompareVersions(left, right string) int {
	a, b := normalizeVersion(left), normalizeVersion(right)
	if a == b {
		return 0
	}
	aParts, bParts := strings.Split(a, "."), strings.Split(b, ".")
	limit := len(aParts)
	if len(bParts) > limit {
		limit = len(bParts)
	}
	for i := 0; i < limit; i++ {
		aPart, bPart := "0", "0"
		if i < len(aParts) {
			aPart = aParts[i]
		}
		if i < len(bParts) {
			bPart = bParts[i]
		}
		if cmp := comparePart(aPart, bPart); cmp != 0 {
			return cmp
		}
	}
	// All numeric components (and their suffixes) are equal; a shorter version
	// with implicit trailing zeros is the same version.
	return 0
}

// VersionAtLeast reports whether have is equal or newer than want. Empty versions
// never satisfy a requirement.
func VersionAtLeast(have, want string) bool {
	if strings.TrimSpace(have) == "" || strings.TrimSpace(want) == "" {
		return false
	}
	return CompareVersions(have, want) >= 0
}

func normalizeVersion(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "v")
	value = strings.TrimPrefix(value, "V")
	// Remove a leading localized word commonly present in uninstall entries.
	value = strings.TrimSpace(value)
	if len(value) > 64 {
		value = value[:64]
	}
	return value
}

func comparePart(a, b string) int {
	aNum, aRest := splitNumeric(a)
	bNum, bRest := splitNumeric(b)
	if cmp := compareNumeric(aNum, bNum); cmp != 0 {
		return cmp
	}
	// Numeric prefixes are equal: compare the remaining suffix using prerelease
	// ordering, where an empty suffix (final release) sorts after any suffix.
	aRest, bRest = strings.ToLower(aRest), strings.ToLower(bRest)
	if aRest == bRest {
		return 0
	}
	if aRest == "" {
		return 1
	}
	if bRest == "" {
		return -1
	}
	return strings.Compare(aRest, bRest)
}

func splitNumeric(value string) (string, string) {
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return value[:i], value[i:]
		}
	}
	return value, ""
}

func compareNumeric(a, b string) int {
	a = strings.TrimLeft(a, "0")
	b = strings.TrimLeft(b, "0")
	if a == b {
		return 0
	}
	if len(a) != len(b) {
		if len(a) < len(b) {
			return -1
		}
		return 1
	}
	return strings.Compare(a, b)
}
