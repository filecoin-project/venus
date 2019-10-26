package testflags

import (
	"flag"
	"testing"
)

// Test enablement flags
// Only run unit and integration tests by default, all others require their flags to be set.
var integrationTest = flag.Bool("integration", true, "Run the integration go tests")
var unitTest = flag.Bool("unit", true, "Run the unit go tests")
var functionalTest = flag.Bool("functional", false, "Run the functional go tests")
var sectorBuilderTest = flag.Bool("sectorbuilder", false, "Run the sector builder tests")
var deploymentTest = flag.String("deployment", "", "Run the deployment tests against a network")
var binaryPath = flag.String("binary-path", "", "Run forked processes tests using provided binary")

// BinaryPath will return the path to the user provided binary. The call is expected to check if
// the return path points to an actual file. If the user did not provide a value an empty string
// will be returned along with a false for the second return value.
func BinaryPath() (string, bool) {
	if len(*binaryPath) == 0 {
		return "", false
	}

	return *binaryPath, true
}

// DeploymentTest will run the test its called from iff the `-deployment` flag
// is passed when calling `go test`. Otherwise the test will be skipped. DeploymentTest
// will run the test its called from in parallel.
// The network under test will be returned.
func DeploymentTest(t *testing.T) string {
	if len(*deploymentTest) == 0 {
		t.SkipNow()
	}
	t.Parallel()

	return *deploymentTest
}

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

// SectorBuilderTest will run the test its called from iff the `-sectorbuilder` flag
// is passed when calling `go test`. Otherwise the test will be skipped.
func SectorBuilderTest(t *testing.T) {
	if !*sectorBuilderTest {
		t.SkipNow()
	}
}
