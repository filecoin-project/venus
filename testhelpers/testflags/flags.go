package testflags

import (
	"flag"
	"testing"
)

// Test enablement flags
// Only run unit and integration tests by default, all others require their flags to be set.
var functionalTest = flag.Bool("functional", false, "Run the functional go tests")
var integrationTest = flag.Bool("integration", true, "Run the integration go tests")
var unitTest = flag.Bool("unit", true, "Run the unit go tests")

// FunctionalTest will run the test its called from iff the `-functional` flag
// is passed when calling `go test`. Otherwise the test will be skipped. FunctionalTest
// will run the test its called from in parallel.
func FunctionalTest(t *testing.T) {
	if !*functionalTest {
		t.SkipNow()
	}
	t.Parallel()
}

// IntegrationTest will run the test its called from iff the `-integration` flag
// is passed when calling `go test`. Otherwise the test will be skipped. IntegrationTest
// will run the test its called from in parallel.
func IntegrationTest(t *testing.T) {
	if !*integrationTest {
		t.SkipNow()
	}
	t.Parallel()
}

// UnitTest will run the test its called from iff the `-unit` or `-short` flag
// is passed when calling `go test`. Otherwise the test will be skipped. UnitTest
// will run the test its called from in parallel.
func UnitTest(t *testing.T) {
	if !*unitTest && !testing.Short() {
		t.SkipNow()
	}
	t.Parallel()
}

// BadUnitTestWithSideEffects will run the test its called from iff the
// `-unit` or `-short` flag is passed when calling `go test`. Otherwise the test
// will be skipped. BadUnitTestWithSideEffects will run the test its called
// serially. Tests that use this flag are bad an should feel bad.
func BadUnitTestWithSideEffects(t *testing.T) {
	if !*unitTest && !testing.Short() {
		t.SkipNow()
	}
}
