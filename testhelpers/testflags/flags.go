package testflags

import (
	"flag"
	"testing"
)

// Test enablement flags
var functionalTest = flag.Bool("functional", false, "Run the functional go tests")
var integrationTest = flag.Bool("integration", false, "Run the integration go tests")
var unitTest = flag.Bool("unit", false, "Run the unit go tests")

// Test modifiers
// concurrent flag will cause tests to be ran in parallel, its default value is `true`.
var concurrent = flag.Bool("concurrent", true, "Runs the go test in parallel")

// FunctionalTest will run the test its called from iff the `-functional` flag
// is passed when calling `go test`. Otherwise the test will be skipped. FunctionalTest
// will run the test its called from in parallel, to disallow this behaviour see
// the concurrent flag.
func FunctionalTest(t *testing.T) {
	if !*functionalTest {
		t.SkipNow()
	}
	if *concurrent {
		t.Parallel()
	}
}

// IntegrationTest will run the test its called from iff the `-integration` flag
// is passed when calling `go test`. Otherwise the test will be skipped. IntegrationTest
// will run the test its called from in parallel, to disallow this behaviour see
// the concurrent flag.
func IntegrationTest(t *testing.T) {
	if !*integrationTest {
		t.SkipNow()
	}
	if *concurrent {
		t.Parallel()
	}
}

// UnitTest will run the test its called from iff the `-unit` or `-short` flag
// is passed when calling `go test`. Otherwise the test will be skipped. UnitTest
// will run the test its called from in parallel, to disallow this behaviour see
// the concurrent flag.
func UnitTest(t *testing.T) {
	if !*unitTest && !testing.Short() {
		t.SkipNow()
	}
	if *concurrent {
		t.Parallel()
	}
}

// UnitTestWithSideEffectsThatIsBad will run the test its called from iff the
// `-unit` or `-short` flag is passed when calling `go test`. Otherwise the test
// will be skipped. UnitTestWithSideEffectsThatIsBad will run the test its called
// serially. Tests that use this flag are bad an should feel bad.
func UnitTestWithSideEffectsThatIsBad(t *testing.T) {
	if !*unitTest && !testing.Short() {
		t.SkipNow()
	}
}
