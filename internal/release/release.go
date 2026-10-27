// Package release computes the next version tag for a merged pull request.
//
// The bump is read from a "Release: patch|minor|major" line in the pull
// request description and defaults to patch.
package release

import (
	"fmt"
	"regexp"
	"strconv"
)

// Bump is the kind of version increase a release makes.
type Bump int

// Bumps in increasing order of significance.
const (
	Patch Bump = iota
	Minor
	Major
)

// String returns the lower-case name of the bump.
func (b Bump) String() string {
	switch b {
	case Minor:
		return "minor"
	case Major:
		return "major"
	default:
		return "patch"
	}
}

var (
	htmlComment = regexp.MustCompile(`(?s)<!--.*?-->`)
	// releaseLine matches a whole line such as "Release: minor". Allowing \r
	// handles the CRLF line endings GitHub stores for descriptions typed in
	// the browser.
	releaseLine = regexp.MustCompile(`(?im)^[ \t]*release:[ \t]*(major|minor|patch)[ \t\r]*$`)
	versionTag  = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
)

// BumpFromDescription returns the bump requested by a pull request
// description. HTML comments are ignored so a template can list the options
// without selecting one. If several lines are present the most significant
// wins; with none, the result is Patch.
func BumpFromDescription(body string) Bump {
	body = htmlComment.ReplaceAllString(body, "")
	bump := Patch
	for _, m := range releaseLine.FindAllStringSubmatch(body, -1) {
		switch lower(m[1]) {
		case "major":
			bump = max(bump, Major)
		case "minor":
			bump = max(bump, Minor)
		}
	}
	return bump
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if 'A' <= c && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}

// Version is a semantic version without pre-release or build metadata.
type Version struct {
	Major, Minor, Patch int
}

// String renders the version as a tag such as "v1.2.3".
func (v Version) String() string {
	return fmt.Sprintf("v%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Parse parses a tag of the form vMAJOR.MINOR.PATCH. Pre-release tags such as
// v1.0.0-rc1 and tags with leading zeros are rejected.
func Parse(tag string) (Version, bool) {
	m := versionTag.FindStringSubmatch(tag)
	if m == nil {
		return Version{}, false
	}
	n := func(s string) int {
		v, _ := strconv.Atoi(s) // the regexp guarantees digits
		return v
	}
	return Version{n(m[1]), n(m[2]), n(m[3])}, true
}

// Bump returns the version after applying b. A major bump resets minor and
// patch; a minor bump resets patch.
func (v Version) Bump(b Bump) Version {
	switch b {
	case Major:
		return Version{v.Major + 1, 0, 0}
	case Minor:
		return Version{v.Major, v.Minor + 1, 0}
	default:
		return Version{v.Major, v.Minor, v.Patch + 1}
	}
}

// Less reports whether v sorts before w.
func (v Version) Less(w Version) bool {
	if v.Major != w.Major {
		return v.Major < w.Major
	}
	if v.Minor != w.Minor {
		return v.Minor < w.Minor
	}
	return v.Patch < w.Patch
}

// First is the version used when no release tag exists yet.
var First = Version{0, 1, 0}

// Next returns the version that follows the highest valid release tag in tags
// after applying b. Tags that are not plain vMAJOR.MINOR.PATCH are ignored.
// With no valid tag it returns First.
func Next(tags []string, b Bump) Version {
	var latest Version
	found := false
	for _, t := range tags {
		v, ok := Parse(t)
		if !ok {
			continue
		}
		if !found || latest.Less(v) {
			latest, found = v, true
		}
	}
	if !found {
		return First
	}
	return latest.Bump(b)
}
