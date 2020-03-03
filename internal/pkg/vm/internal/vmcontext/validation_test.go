package vmcontext

import (
	"fmt"
	"reflect"
	"runtime"
	"testing"

	"github.com/filecoin-project/chain-validation/suites"
	"github.com/filecoin-project/chain-validation/suites/message"
)

// TestSkipper contains a list of test cases skipped by the implementation.
type TestSkipper struct {
	testSkips []suites.TestCase
}

// Skip return true if the sutire.TestCase should be skipped.
func (ts *TestSkipper) Skip(test suites.TestCase) bool {
	for _, skip := range ts.testSkips {
		if reflect.ValueOf(skip).Pointer() == reflect.ValueOf(test).Pointer() {
			fmt.Printf("=== SKIP   %v\n", runtime.FuncForPC(reflect.ValueOf(test).Pointer()).Name())
			return true
		}
	}
	return false
}

// TestSuiteSkips contains tests we wish to skip.
var TestSuiteSkipper TestSkipper

func init() {
	// initialize the test skipper with tests being skipped
	TestSuiteSkipper = TestSkipper{testSkips: []suites.TestCase{
		// Skipping since multisig address resolution breaks tests
		// https://github.com/filecoin-project/specs-actors/issues/184
		message.TestMultiSigActor,
	}}
}

func TestChainValidationMessageSuite(t *testing.T) {
	f := NewFactories()
	for _, testCase := range suites.MessageTestCases() {
		if TestSuiteSkipper.Skip(testCase) {
			continue
		}
		testCase(t, f)
	}
}

func TestChainValidationTipSetSuite(t *testing.T) {
	f := NewFactories()
	for _, testCase := range suites.TipSetTestCases() {
		if TestSuiteSkipper.Skip(testCase) {
			continue
		}
		testCase(t, f)
	}
}
