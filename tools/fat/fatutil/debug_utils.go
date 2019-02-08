package fastutil

import (
	"encoding/json"
	"fmt"
	"github.com/ipfs/iptb/testbed/interfaces"
	"io"
	"io/ioutil"
	"strings"
)

// Output represent the output of a command run
type Output struct {
	Args     []string
	ExitCode int
	Error    error
	Stderr   string
	Stdout   string
}

// DumpOutput prints the output to the io.Writer in a human readable way
func DumpOutput(w io.Writer, output testbedi.Output) {
	fmt.Fprintf(w, ">>>> start-dump\n")                                       // nolint: errcheck
	fmt.Fprintf(w, "---- command     %s\n", strings.Join(output.Args(), " ")) // nolint: errcheck
	fmt.Fprintf(w, "---- exit-code   %d\n", output.ExitCode())                // nolint: errcheck

	err := output.Error()
	if err != nil {
		fmt.Fprintf(w, "---- error       %s\n", output.Error()) // nolint: errcheck
	} else {
		fmt.Fprintf(w, "---- error       nil\n") // nolint: errcheck
	}

	fmt.Fprintf(w, "---- stdout\n")   // nolint: errcheck
	io.Copy(w, output.Stdout())       // nolint: errcheck
	fmt.Fprintf(w, "---- stderr\n")   // nolint: errcheck
	io.Copy(w, output.Stderr())       // nolint: errcheck
	fmt.Fprintf(w, "<<<< end-dump\n") // nolint: errcheck
}

// DumpOutputJSON prints the json encoded output to the io.Writer
func DumpOutputJSON(w io.Writer, output testbedi.Output) {
	stdout, _ := ioutil.ReadAll(output.Stdout()) // nolint: errcheck
	stderr, _ := ioutil.ReadAll(output.Stderr()) // nolint: errcheck

	jout := &Output{
		Args:     output.Args(),
		ExitCode: output.ExitCode(),
		Error:    output.Error(),
		Stderr:   string(stderr),
		Stdout:   string(stdout),
	}

	enc := json.NewEncoder(w)
	_ = enc.Encode(jout) // nolint: errcheck
}
