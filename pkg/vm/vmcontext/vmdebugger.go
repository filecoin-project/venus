package vmcontext

import (
	"fmt"
	"io/ioutil"
	"strings"
)

type VMDebugMsg struct {
	buf *strings.Builder
}

func NewVMDebugMsg() *VMDebugMsg {
	return &VMDebugMsg{buf: &strings.Builder{}}
}

func (debug *VMDebugMsg) Printfln(msg string, args ...interface{}) {
	debug.buf.WriteString(fmt.Sprintf(msg, args...))
	debug.buf.WriteString("\n")
}

func (debug *VMDebugMsg) Println(args ...interface{}) {
	debug.buf.WriteString(fmt.Sprint(args...))
	debug.buf.WriteString("\n")
}

func (debug *VMDebugMsg) WriteToTerminal() {
	fmt.Println(debug.buf.String())
}

func (debug *VMDebugMsg) WriteToFile(fileName string) error {
	return ioutil.WriteFile(fileName, []byte(debug.buf.String()), 0777)
}
