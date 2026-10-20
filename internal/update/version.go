// Package update implements the self update capability of the panel.
//
// The panel asks a release feed which versions exist, compares them with
// the running version and, when asked to, downloads the matching artefact,
// verifies its checksum, replaces the running binary and optionally asks
// the service manager to restart it.
package update

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a release version. The format is the usual major.minor.patch
// with an optional pre release suffix.
type Version struct {
	Major int
	Minor int
	Patch int
	Pre   string
}

// Parse reads a version string. A leading "v" is allowed.
func Parse(raw string) (Version, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return Version{}, fmt.Errorf("update: empty version")
	}
	text = strings.TrimPrefix(text, "v")
	if idx := strings.IndexAny(text, "+"); idx >= 0 {
		text = text[:idx]
	}

	pre := ""
	if idx := strings.Index(text, "-"); idx >= 0 {
		pre = text[idx+1:]
		text = text[:idx]
	}

	parts := strings.Split(text, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return Version{}, fmt.Errorf("update: invalid version %q", raw)
	}

	version := Version{Pre: pre}
	targets := []*int{&version.Major, &version.Minor, &version.Patch}
	for i, part := range parts {
		if i >= len(targets) {
			break
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			return Version{}, fmt.Errorf("update: invalid version %q", raw)
		}
		*targets[i] = value
	}
	return version, nil
}

// String renders the version.
func (v Version) String() string {
	base := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		base += "-" + v.Pre
	}
	return base
}

// IsUnknown reports whether the version is a development build.
func (v Version) IsUnknown() bool {
	return v.Major == 0 && v.Minor == 0 && v.Patch == 0 && v.Pre == ""
}

// Compare returns -1 when v is older than other, 1 when it is newer and 0
// when both are equal. A pre release is always older than the release it
// belongs to.
func (v Version) Compare(other Version) int {
	switch {
	case v.Major != other.Major:
		return sign(v.Major - other.Major)
	case v.Minor != other.Minor:
		return sign(v.Minor - other.Minor)
	case v.Patch != other.Patch:
		return sign(v.Patch - other.Patch)
	}

	switch {
	case v.Pre == other.Pre:
		return 0
	case v.Pre == "":
		return 1
	case other.Pre == "":
		return -1
	case v.Pre < other.Pre:
		return -1
	default:
		return 1
	}
}

// IsNewer reports whether v is a newer release than other.
func (v Version) IsNewer(other Version) bool { return v.Compare(other) > 0 }

// Normalise returns the version without the leading "v".
func Normalise(raw string) string {
	return strings.TrimPrefix(strings.TrimSpace(raw), "v")
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
