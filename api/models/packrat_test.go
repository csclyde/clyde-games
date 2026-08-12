package models

import "testing"

func TestCompareSemanticVersions(t *testing.T) {
	cases := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "same", left: "0.1.0", right: "0.1.0", want: 0},
		{name: "patch behind", left: "0.1.0", right: "0.1.1", want: -1},
		{name: "minor behind", left: "0.1.9", right: "0.2.0", want: -1},
		{name: "major ahead", left: "1.0.0", right: "0.9.9", want: 1},
		{name: "prerelease behind release", left: "1.0.0-beta", right: "1.0.0", want: -1},
		{name: "build metadata ignored", left: "1.0.0+build.1", right: "1.0.0", want: 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := compareSemanticVersions(tc.left, tc.right)
			if (got < 0 && tc.want >= 0) || (got == 0 && tc.want != 0) || (got > 0 && tc.want <= 0) {
				t.Fatalf("compareSemanticVersions(%q, %q) = %d, want sign %d", tc.left, tc.right, got, tc.want)
			}
		})
	}
}
