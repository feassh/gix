package version

import (
	"fmt"
	"strconv"
	"strings"
)

type SemanticVersion struct {
	Major int
	Minor int
	Patch int
}

func ParseTag(tag string, prefix string) (SemanticVersion, error) {
	raw := strings.TrimSpace(tag)
	if raw == "" {
		return SemanticVersion{}, fmt.Errorf("empty tag")
	}
	if prefix != "" {
		raw = strings.TrimPrefix(raw, prefix)
	}
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return SemanticVersion{}, fmt.Errorf("tag %q is not semver", tag)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return SemanticVersion{}, err
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return SemanticVersion{}, err
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return SemanticVersion{}, err
	}
	return SemanticVersion{Major: major, Minor: minor, Patch: patch}, nil
}

func NextTag(current string, prefix string, increment string) (string, error) {
	if strings.TrimSpace(current) == "" {
		return prefix + "0.1.0", nil
	}
	ver, err := ParseTag(current, prefix)
	if err != nil {
		return "", err
	}
	switch strings.ToLower(strings.TrimSpace(increment)) {
	case "", "patch":
		ver.Patch++
	case "minor":
		ver.Minor++
		ver.Patch = 0
	case "major":
		ver.Major++
		ver.Minor = 0
		ver.Patch = 0
	default:
		return "", fmt.Errorf("unsupported increment %q", increment)
	}
	return fmt.Sprintf("%s%d.%d.%d", prefix, ver.Major, ver.Minor, ver.Patch), nil
}

func CompareTags(left string, right string, prefix string) (int, error) {
	lv, err := ParseTag(addPrefix(left, prefix), prefix)
	if err != nil {
		return 0, err
	}
	rv, err := ParseTag(addPrefix(right, prefix), prefix)
	if err != nil {
		return 0, err
	}
	switch {
	case lv.Major != rv.Major:
		if lv.Major < rv.Major {
			return -1, nil
		}
		return 1, nil
	case lv.Minor != rv.Minor:
		if lv.Minor < rv.Minor {
			return -1, nil
		}
		return 1, nil
	case lv.Patch != rv.Patch:
		if lv.Patch < rv.Patch {
			return -1, nil
		}
		return 1, nil
	default:
		return 0, nil
	}
}

func addPrefix(value string, prefix string) string {
	raw := strings.TrimSpace(value)
	if prefix == "" || strings.HasPrefix(raw, prefix) {
		return raw
	}
	return prefix + raw
}
