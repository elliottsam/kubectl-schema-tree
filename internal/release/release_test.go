package release_test

import (
	"testing"

	"github.com/elliottsam/kubectl-schema-tree/internal/release"
)

func TestBumpFromDescription(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		body string
		want release.Bump
	}{
		{"empty", "", release.Patch},
		{"no-line", "Fixes a bug.\n\nTest plan: ran it.", release.Patch},
		{"patch", "Release: patch", release.Patch},
		{"minor", "Summary\n\nRelease: minor\n", release.Minor},
		{"major", "Release: major", release.Major},
		{"case-insensitive", "release: MINOR", release.Minor},
		{"indented", "  Release:   major  ", release.Major},
		{"crlf", "## Release\r\nRelease: minor\r\n", release.Minor},
		{"highest-wins", "Release: minor\nRelease: major\nRelease: patch", release.Major},
		{"comment-ignored", "<!-- Release: major -->\nRelease: patch", release.Patch},
		{"multiline-comment-ignored", "<!--\nRelease: major\nRelease: minor\n-->\n", release.Patch},
		{"mentioned-in-prose", "This is not a major release: minor changes only.", release.Patch},
		{"unknown-value", "Release: huge", release.Patch},
		{"extra-text-after", "Release: minor because reasons", release.Patch},
		{"prefix-text", "Next Release: major", release.Patch},
		{"none", "Release: none", release.None},
		{"none-case-insensitive", "RELEASE: None\r\n", release.None},
		{"none-overrides-major", "Release: major\nRelease: none", release.None},
		{"none-before-major", "Release: none\nRelease: major", release.None},
		{"none-in-comment-ignored", "<!-- Release: none -->\nRelease: minor", release.Minor},
		{"none-in-prose", "There is none to release: none of it.", release.Patch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := release.BumpFromDescription(tt.body); got != tt.want {
				t.Fatalf("BumpFromDescription(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Parallel()
	tests := []struct {
		tag  string
		want release.Version
		ok   bool
	}{
		{"v0.1.0", release.Version{0, 1, 0}, true},
		{"v1.22.333", release.Version{1, 22, 333}, true},
		{"1.2.3", release.Version{}, false},
		{"v1.2", release.Version{}, false},
		{"v1.2.3-rc1", release.Version{}, false},
		{"v1.2.3+build", release.Version{}, false},
		{"v01.2.3", release.Version{}, false},
		{"vx.y.z", release.Version{}, false},
		{"", release.Version{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			t.Parallel()
			got, ok := release.Parse(tt.tag)
			if ok != tt.ok || got != tt.want {
				t.Fatalf("Parse(%q) = %v, %v; want %v, %v", tt.tag, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tags []string
		bump release.Bump
		want string
	}{
		{"no-tags", nil, release.Patch, "v0.1.0"},
		{"no-valid-tags", []string{"latest", "v1.0.0-rc1"}, release.Major, "v0.1.0"},
		{"patch", []string{"v0.1.0"}, release.Patch, "v0.1.1"},
		{"minor-resets-patch", []string{"v0.1.4"}, release.Minor, "v0.2.0"},
		{"major-resets-all", []string{"v0.3.7"}, release.Major, "v1.0.0"},
		{"picks-highest-numerically", []string{"v0.9.0", "v0.10.0", "v0.2.0"}, release.Patch, "v0.10.1"},
		{"order-independent", []string{"v1.0.0", "v0.9.9"}, release.Minor, "v1.1.0"},
		{"ignores-prerelease-above", []string{"v1.2.3", "v2.0.0-rc1"}, release.Patch, "v1.2.4"},
		{"none-keeps-version", []string{"v1.2.3"}, release.None, "v1.2.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := release.Next(tt.tags, tt.bump).String(); got != tt.want {
				t.Fatalf("Next(%v, %v) = %s, want %s", tt.tags, tt.bump, got, tt.want)
			}
		})
	}
}

func TestBumpString(t *testing.T) {
	t.Parallel()
	for b, want := range map[release.Bump]string{release.Patch: "patch", release.Minor: "minor", release.Major: "major", release.None: "none"} {
		if got := b.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", b, got, want)
		}
	}
}

func FuzzBumpFromDescription(f *testing.F) {
	f.Add("Release: minor")
	f.Add("<!-- Release: major -->")
	f.Add("\r\n\r\nrelease:\tmajor\r\n")
	f.Fuzz(func(t *testing.T, body string) {
		if b := release.BumpFromDescription(body); b < release.Patch || b > release.None {
			t.Fatalf("out-of-range bump %d", b)
		}
	})
}
