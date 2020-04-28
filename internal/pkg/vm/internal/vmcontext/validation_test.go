package vmcontext

import (
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/filecoin-project/chain-validation/suites"
	"github.com/filecoin-project/chain-validation/suites/tipset"
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
		tipset.TestBlockMessageDeduplication,
		tipset.TestMinerRewardsAndPenalties,
	}}
}

func TestChainValidationMessageSuite(t *testing.T) {
	f := NewFactories(&ValidationConfig{
		trackGas:         true,
		checkExitCode:    true,
		checkReturnValue: true,
		checkStateRoot:   true,
	})
	for _, testCase := range suites.MessageTestCases() {
		if TestSuiteSkipper.Skip(testCase) {
			continue
		}
		t.Run(caseName(testCase), func(t *testing.T) {
			testCase(t, f)
		})
	}
}

func TestChainValidationTipSetSuite(t *testing.T) {
	f := NewFactories(&ValidationConfig{
		trackGas:         true,
		checkExitCode:    true,
		checkReturnValue: true,
		checkStateRoot:   true,
	})
	for _, testCase := range suites.TipSetTestCases() {
		if TestSuiteSkipper.Skip(testCase) {
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
