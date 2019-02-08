package fastutil

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/pmezard/go-difflib/difflib"
)

func writeLine(seed int, ws ...io.Writer) error {
	alphabet := "0123456789ABCDEF"
	var line strings.Builder
	for x := 0; x < 64; x++ {
		c := alphabet[(x+seed)%len(alphabet)]
		line.WriteByte(c)
	}

	line.WriteByte('\n')

	for _, w := range ws {
		n, err := w.Write([]byte(line.String()))

		if err != nil {
			return err
		}

		if n != line.Len() {
			return fmt.Errorf("did not write entire line")
		}

	}

	return nil
}

func compare(t *testing.T, expected, actual []byte) {
	if !bytes.Equal(expected, actual) {
		diff := difflib.UnifiedDiff{
			A:        difflib.SplitLines(string(expected)),
			B:        difflib.SplitLines(string(actual)),
			FromFile: "Expected",
			ToFile:   "Actual",
			Context:  3,
		}
		text, _ := difflib.GetUnifiedDiffString(diff)

		t.Logf("\n%s", text)
		t.Fatal("data does not match")
	}
}

func writeLines(seed int, num int, ws ...io.Writer) error {
	for i := 0; i < num; i++ {
		if err := writeLine(seed+i, ws...); err != nil {
			return err
		}
	}

	return nil
}
