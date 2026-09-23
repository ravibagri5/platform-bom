// Package version parses the loosely semver-shaped versions found in the wild
// (image tags, Helm appVersions, Git tags, Kubernetes gitVersions).
package version

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	versionRE    = regexp.MustCompile(`^[vV]?(\d+)(?:\.(\d+))?(?:\.(\d+))?(?:[-+._]?(.*))?$`)
	prereleaseRE = regexp.MustCompile(`(?i)(^|[.\-_])(alpha|beta|rc|pre|preview|dev|snapshot|nightly)`)
)

// V is a parsed version.
type V struct {
	Major, Minor, Patch int
	// Parts is how many numeric components were present (1-3).
	Parts  int
	Suffix string
}

// Parse parses s leniently. It returns false when s does not start with a number.
func Parse(s string) (V, bool) {
	m := versionRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return V{}, false
	}
	v := V{Parts: 1, Suffix: m[4]}
	v.Major, _ = strconv.Atoi(m[1])
	if m[2] != "" {
		v.Minor, _ = strconv.Atoi(m[2])
		v.Parts = 2
	}
	if m[3] != "" {
		v.Patch, _ = strconv.Atoi(m[3])
		v.Parts = 3
	}
	return v, true
}

// String returns the numeric core, e.g. "1.34.2".
func (v V) String() string {
	switch v.Parts {
	case 1:
		return strconv.Itoa(v.Major)
	case 2:
		return fmt.Sprintf("%d.%d", v.Major, v.Minor)
	default:
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	}
}

// Prerelease reports whether the suffix marks an alpha/beta/rc style build.
func (v V) Prerelease() bool {
	return v.Suffix != "" && prereleaseRE.MatchString(v.Suffix)
}

// Display is String plus a prerelease suffix, e.g. "2.0.1-rc.3". Distribution suffixes are dropped.
func (v V) Display() string {
	if v.Prerelease() {
		return v.String() + "-" + v.Suffix
	}
	return v.String()
}

// Compare orders versions by numeric core, placing prereleases before releases.
func Compare(a, b V) int {
	for _, d := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if d[0] != d[1] {
			if d[0] < d[1] {
				return -1
			}
			return 1
		}
	}
	switch {
	case a.Prerelease() && !b.Prerelease():
		return -1
	case !a.Prerelease() && b.Prerelease():
		return 1
	}
	return 0
}

// CompareStrings compares two version strings; unparseable versions sort first.
func CompareStrings(a, b string) int {
	va, oka := Parse(a)
	vb, okb := Parse(b)
	switch {
	case !oka && !okb:
		return strings.Compare(a, b)
	case !oka:
		return -1
	case !okb:
		return 1
	}
	return Compare(va, vb)
}

// Normalize returns the numeric core of s, or s unchanged if it cannot be parsed.
func Normalize(s string) string {
	if v, ok := Parse(s); ok {
		return v.String()
	}
	return s
}

// Matches reports whether actual satisfies declared, comparing only the
// components present in declared ("1.34" matches "1.34.2").
func Matches(declared, actual string) bool {
	if declared == "" || actual == "" {
		return declared == actual
	}
	d, okd := Parse(declared)
	a, oka := Parse(actual)
	if !okd || !oka {
		return declared == actual
	}
	if d.Major != a.Major {
		return false
	}
	if d.Parts >= 2 && d.Minor != a.Minor {
		return false
	}
	if d.Parts >= 3 && d.Patch != a.Patch {
		return false
	}
	if d.Parts >= 3 && (d.Prerelease() || a.Prerelease()) {
		return d.Suffix == a.Suffix
	}
	return true
}
