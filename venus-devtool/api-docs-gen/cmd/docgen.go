package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"sort"
	"strings"

	"github.com/filecoin-project/go-address"
	docgen "github.com/filecoin-project/venus/venus-devtool/api-docs-gen"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func main() {
	comments, groupComments := docgen.ParseApiASTInfo(os.Args[1], os.Args[2], os.Args[3], os.Args[4])

	groups := make(map[string]*docgen.MethodGroup)

	_, t, permStruct := docgen.GetAPIType(os.Args[2], os.Args[3])

	for i := 0; i < t.NumMethod(); i++ {
		m := t.Method(i)

		groupName := docgen.MethodGroupFromName(m.Name)

		g, ok := groups[groupName]
		if !ok {
			g = new(docgen.MethodGroup)
			g.Header = groupComments[groupName]
			g.GroupName = groupName
			groups[groupName] = g
		}
		var args []interface{}
		ft := m.Func.Type()
		for j := 2; j < ft.NumIn(); j++ {
			inp := ft.In(j)
			args = append(args, docgen.ExampleValue(m.Name, inp, nil))
		}

		v, err := json.MarshalIndent(args, "", "  ")
		if err != nil {
			panic(err)
		}

		outv := docgen.ExampleValue(m.Name, ft.Out(0), nil)
		if outv == nil {
			_, _ = fmt.Fprintf(os.Stderr, "ExampleValue for %s get nil\n", ft.Out(0).String())
			continue
		}
		// json: unsupported type: map[address.Address]*types.Actor, so use map[string]*types.Actor instead
		if actors, ok := outv.(map[address.Address]*types.Actor); ok {
			newActors := make(map[string]*types.Actor, len(actors))
			for addr, a := range actors {
				newActors[addr.String()] = a
			}
			outv = newActors
		}

		ov, err := json.MarshalIndent(outv, "", "  ")
		if err != nil {
			panic(err)
		}

		g.Methods = append(g.Methods, &docgen.Method{
			Name:            m.Name,
			Comment:         comments[m.Name],
			InputExample:    string(v),
			ResponseExample: string(ov),
		})
	}

	var groupslice []*docgen.MethodGroup
	for _, g := range groups {
		groupslice = append(groupslice, g)
	}

	sort.Slice(groupslice, func(i, j int) bool {
		return groupslice[i].GroupName < groupslice[j].GroupName
	})

	buf := &bytes.Buffer{}
	fmt.Fprint(buf, "# Groups\n")

	for _, g := range groupslice {
		fmt.Fprintf(buf, "* [%s](#%s)\n", g.GroupName, g.GroupName)
		for _, method := range g.Methods {
			fmt.Fprintf(buf, "  * [%s](#%s)\n", method.Name, method.Name)
		}
	}

	for _, g := range groupslice {
		g := g
		fmt.Fprintf(buf, "## %s\n", g.GroupName)
		fmt.Fprintf(buf, "%s\n\n", g.Header)

		sort.Slice(g.Methods, func(i, j int) bool {
			return g.Methods[i].Name < g.Methods[j].Name
		})

		for _, m := range g.Methods {
			fmt.Fprintf(buf, "### %s\n", m.Name)
			fmt.Fprintf(buf, "%s\n\n", m.Comment)

			var meth reflect.StructField
			var ok bool
			for _, ps := range permStruct {
				meth, ok = ps.FieldByName(m.Name)
				if ok {
					break
				}
			}
			if !ok {
				panic("no perms for method: " + m.Name)
			}

			perms := meth.Tag.Get("perm")

			fmt.Fprintf(buf, "Perms: %s\n\n", perms)

			if strings.Count(m.InputExample, "\n") > 0 {
				fmt.Fprintf(buf, "Inputs:\n```json\n%s\n```\n\n", m.InputExample)
			} else {
				fmt.Fprintf(buf, "Inputs: `%s`\n\n", m.InputExample)
			}

			if strings.Count(m.ResponseExample, "\n") > 0 {
				fmt.Fprintf(buf, "Response:\n```json\n%s\n```\n\n", m.ResponseExample)
			} else {
				fmt.Fprintf(buf, "Response: `%s`\n\n", m.ResponseExample)
			}
		}
	}

	if err := ioutil.WriteFile(os.Args[5], buf.Bytes(), 0644); err != nil {
		panic(err)
	}
}
