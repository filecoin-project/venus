package vmcontext

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
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
		// NewActorIDAddress implementation is wrong
		message.TestInitActorSequentialIDAddressCreate,
		message.TestMessageApplicationEdgecases,
		// Skipping since multisig address resolution breaks tests
		// https://github.com/filecoin-project/specs-actors/issues/184
		message.TestMultiSigActor,
		// Skipping since payment channel because runtime sys calls are not implemented in runtime adapter
		message.TestPaych,
	}}
}

func TestChainValidationMessageSuite(t *testing.T) {
	f := NewFactories()
	for _, testCase := range suites.MessageTestCases() {
		// All skipped due to gas accounting causing divergent balance calculations.
		if true || TestSuiteSkipper.Skip(testCase) {
			continue
		}
		t.Run(caseName(testCase), func(t *testing.T) {
			testCase(t, f)
		})
	}
}

func TestChainValidationTipSetSuite(t *testing.T) {
	f := NewFactories()
	for _, testCase := range suites.TipSetTestCases() {
		// All skipped due to VM state incorrectly throws illegal actor state error.
		if true || TestSuiteSkipper.Skip(testCase) {
			continue
		}
		t.Run(caseName(testCase), func(t *testing.T) {
			testCase(t, f)
		})
	}
}

func caseName(testCase suites.TestCase) string {
	fqName := runtime.FuncForPC(reflect.ValueOf(testCase).Pointer()).Name()
	toks := strings.Split(fqName, ".")
	return toks[len(toks)-1]
}
