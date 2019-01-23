package fatutil

import (
	"fmt"
	"github.com/ipfs/iptb/testbed/interfaces"
	"io"
	"strings"
)

func DumpOutput(w io.Writer, output testbedi.Output) {
	fmt.Fprintf(w, ">>>> start-dump\n")
	fmt.Fprintf(w, "---- command     %s\n", strings.Join(output.Args(), " "))
	fmt.Fprintf(w, "---- exit-code   %d\n", output.ExitCode())

	err := output.Error()
	if err != nil {
		fmt.Fprintf(w, "---- error       %s\n", output.Error())
	} else {
		fmt.Fprintf(w, "---- error       nil\n")
	}

	fmt.Fprintf(w, "---- stdout\n")
	io.Copy(w, output.Stdout())
	fmt.Fprintf(w, "---- stderr\n")
	io.Copy(w, output.Stderr())
	fmt.Fprintf(w, "<<<< end-dump\n")
}
