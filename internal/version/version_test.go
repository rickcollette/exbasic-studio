package version

import "testing"

func TestDisplayIncludesBuildNumber(t *testing.T) {
	oldVersion, oldBuild := Version, Build
	t.Cleanup(func() { Version, Build = oldVersion, oldBuild })

	Version, Build = "1.0.0", "7"
	if got := Display(); got != "1.0.0 (build 7)" {
		t.Fatalf("Display() = %q", got)
	}
}

func TestDisplayOmitsDevelopmentBuild(t *testing.T) {
	oldVersion, oldBuild := Version, Build
	t.Cleanup(func() { Version, Build = oldVersion, oldBuild })

	Version, Build = "0.0.0-dev", "0"
	if got := Display(); got != "0.0.0-dev" {
		t.Fatalf("Display() = %q", got)
	}
}
