// Package toolstest gives integration tests access to the real bundled
// toolchain.
//
// Point EXBASIC_STUDIO_TOOLS at a staged tool directory to run them. Without
// it the tests skip, unless EXBASIC_STUDIO_REQUIRE_TOOLS=1 is set: release
// and platform CI jobs set that so a missing toolchain is a failure, never a
// silent skip.
package toolstest

import (
	"os"
	"testing"

	"exbasicstudio/internal/tools"
)

// RequireEnv makes a missing toolchain fatal instead of a skip.
const RequireEnv = "EXBASIC_STUDIO_REQUIRE_TOOLS"

func unavailable(t testing.TB, what string) {
	t.Helper()
	if os.Getenv(RequireEnv) == "1" {
		t.Fatalf("%s is unavailable and %s=1; stage the toolchain and set %s", what, RequireEnv, tools.OverrideEnv)
	}
	t.Skipf("%s is unavailable; set %s to a staged tool directory to run this test", what, tools.OverrideEnv)
}

// Tool returns the path of a bundled executable.
func Tool(t testing.TB, name string) string {
	t.Helper()
	path, ok := tools.Bundled(name)
	if !ok {
		unavailable(t, "bundled tool "+name)
	}
	return path
}

// Asset returns the path of a bundled data file.
func Asset(t testing.TB, name string) string {
	t.Helper()
	path, ok := tools.Asset(name)
	if !ok {
		unavailable(t, "bundled asset "+name)
	}
	return path
}
