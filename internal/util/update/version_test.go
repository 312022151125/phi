package update_test

import (
	"testing"

	"github.com/pulseaiclub/phi/internal/util/update"
)

func TestVersionLess(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"v0.1.0", "v0.2.0", true},
		{"0.1.0", "v0.2.0", true},
		{"v0.2.0", "v0.1.0", false},
		{"v0.2.0", "v0.2.0", false},
		{"v0.1.0 (abc)", "v0.1.1", true},
	}
	for _, tc := range cases {
		if got := update.VersionLess(tc.a, tc.b); got != tc.want {
			t.Fatalf("VersionLess(%q, %q)=%v want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestIsDevBuild(t *testing.T) {
	for _, v := range []string{"", "dev", "DEV", "dev-c788df9", "DEV-C788DF9", "0.0.0", "v0.0.0"} {
		if !update.IsDevBuild(v) {
			t.Fatalf("IsDevBuild(%q)=false, want true", v)
		}
	}
	for _, v := range []string{"v0.1.0", "v0.19.0", "development"} {
		if update.IsDevBuild(v) {
			t.Fatalf("IsDevBuild(%q)=true, want false", v)
		}
	}
}
