package testgflags

import (
	"flag"
	"testing"
)

// Commit is the current git commit, injected through ldflags.
var Commit string

var integrationTest = flag.Bool("integration", false, "Run the integration go tests")
var unitTest = flag.Bool("unit", false, "Run the unit go tests")

var concurrent = flag.Bool("concurrent", true, "Run the go tests concurrently")

// IntegrationTest will skip the test its called from if the -integration flag
// is not passed to the testing call.
func IntegrationTest(t *testing.T) {
	if !*integrationTest {
		t.SkipNow()
	}
	if *concurrent {
		t.Parallel()
	}
}

// UnitTest will skip the test its called from if the -unit flag
// is not passed to the testing call.
func UnitTest(t *testing.T) {
	if !*unitTest {
		t.SkipNow()
	}
	if *concurrent {
		t.Parallel()
	}
}
