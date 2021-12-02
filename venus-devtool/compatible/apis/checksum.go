package main

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"reflect"
	"strings"

	"github.com/filecoin-project/lotus/api/v0api"
	"github.com/filecoin-project/lotus/api/v1api"
	"github.com/urfave/cli/v2"
)

var checksumCmd = &cli.Command{
	Name:  "checksum",
	Flags: []cli.Flag{},
	Action: func(cctx *cli.Context) error {
		wants := []reflect.Type{
			reflect.TypeOf((*v0api.FullNode)(nil)).Elem(),
			reflect.TypeOf((*v1api.FullNode)(nil)).Elem(),
		}

		var buf bytes.Buffer
		for _, rt := range wants {
			fmt.Printf("%s:\n", rt)
			for mi := 0; mi < rt.NumMethod(); mi++ {
				buf.Reset()
				meth := rt.Method(mi)
				numIn := meth.Type.NumIn()
				numOut := meth.Type.NumOut()

				for ii := 0; ii < numIn; ii++ {
					inTyp := meth.Type.In(ii)
					fmt.Fprintf(&buf, "\tIn: %s\n", formatType(inTyp)) // nolint
				}

				for oi := 0; oi < numOut; oi++ {
					outTyp := meth.Type.Out(oi)
					fmt.Fprintf(&buf, "\tOut: %s\n", formatType(outTyp)) // nolint
				}

				fmt.Printf("\t%s:\tIn=%d,\tOut=%d,\tCheckSum=%x\n", meth.Name, numIn, numOut, md5.Sum(buf.Bytes()))
			}
			fmt.Println()
		}
		return nil
	},
}

func formatType(rt reflect.Type) string {
	switch rt.Kind() {
	case reflect.Array:
		return fmt.Sprintf("[%d]%s", rt.Len(), formatType(rt.Elem()))

	case reflect.Chan:
		return fmt.Sprintf("%s %s", rt.ChanDir(), formatType(rt.Elem()))

	case reflect.Func:
		ins := make([]string, rt.NumIn())
		outs := make([]string, rt.NumOut())
		for i := range ins {
			ins[i] = formatType(rt.In(i))
		}

		for i := range outs {
			outs[i] = formatType(rt.Out(i))
		}

		return fmt.Sprintf("func(%s) (%s)", strings.Join(ins, ", "), strings.Join(outs, ", "))

	case reflect.Map:
		return fmt.Sprintf("map[%s]%s", formatType(rt.Key()), formatType(rt.Elem()))

	case reflect.Ptr:
		return fmt.Sprintf("*%s", formatType(rt.Elem()))

	case reflect.Slice:
		return fmt.Sprintf("[]%s", formatType(rt.Elem()))

	default:
		if p := rt.PkgPath(); p != "" {
			return p + "." + rt.Name()
		}
		return rt.Name()
	}
}
