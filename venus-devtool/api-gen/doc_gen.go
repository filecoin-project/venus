package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"text/template"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/venus/venus-devtool/api-gen/common"
	"github.com/filecoin-project/venus/venus-devtool/util"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/urfave/cli/v2"
)

var ctxElem = reflect.TypeOf((*context.Context)(nil)).Elem()

var docGenCmd = &cli.Command{
	Name: "doc",
	Action: func(cctx *cli.Context) error {
		if err := util.LoadExtraInterfaceMeta(); err != nil {
			return err
		}
		for _, t := range common.ApiTargets {
			if err := genDocForAPI(t); err != nil {
				return err
			}
		}

		return nil
	},
}

type MethodGroup struct {
	GroupName string
	Methods   []*Method
}

type Method struct {
	Name            string
	Comment         string
	Perm            string
	InputExample    string
	ResponseExample string
}

func genDocForAPI(t util.APIMeta) error {
	opt := t.ParseOpt
	opt.ResolveImports = true
	ifaceMetas, astMeta, err := util.ParseInterfaceMetas(opt)
	if err != nil {
		return err
	}

	groups := make([]MethodGroup, 0, len(ifaceMetas))
	for _, im := range ifaceMetas {
		mg := MethodGroup{GroupName: simpleGroupName(im.Name), Methods: make([]*Method, 0, len(im.Defined))}
		for _, mm := range im.Defined {
			method, ok := t.Type.MethodByName(mm.Name)
			if !ok {
				fmt.Println("not found method: ", mm.Name)
				continue
			}
			in, out, err := fillExampleValue(method)
			if err != nil {
				return err
			}

			m := &Method{
				Comment:         getComment(mm.Comments),
				Name:            mm.Name,
				InputExample:    string(in),
				ResponseExample: string(out),
				Perm:            util.GetAPIMethodPerm(mm),
			}
			mg.Methods = append(mg.Methods, m)
		}
		if len(mg.Methods) == 0 {
			continue
		}
		groups = append(groups, mg)
	}

	return writeAPIInfo(astMeta, t.RPCMeta, groups)
}

func simpleGroupName(groupName string) string {
	// `IBlockStore` ==> `BlockStore`
	// `IJwtAuthAPI` ==> `JwtAuth`
	if len(groupName) > 0 && groupName[0] == 'I' {
		groupName = groupName[1:]
	}
	groupName = strings.Replace(groupName, "API", "", 1)

	return groupName
}

func fillExampleValue(m reflect.Method) ([]byte, []byte, error) {
	ft := m.Type

	in := make([]interface{}, 0, ft.NumIn())
	for i := 0; i < ft.NumIn(); i++ {
		if ft.In(i).Implements(ctxElem) {
			continue
		}
		inp := ft.In(i)
		in = append(in, ExampleValue(m.Name, inp, nil))
	}

	inVal, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	out := ExampleValue(m.Name, ft.Out(0), nil)
	if out == nil {
		return nil, nil, fmt.Errorf("ExampleValue for %s get nil", ft.Out(0).String())
	}
	// json: unsupported type: map[address.Address]*types.Actor, so return {}
	if _, ok := out.(map[address.Address]*types.Actor); ok {
		return inVal, []byte{'{', '}'}, nil
	}

	outVal, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, nil, err
	}

	return inVal, outVal, nil
}

func getComment(comments []*ast.CommentGroup) string {
	// skip permission comment
	if len(comments) == 1 {
		return ""
	}
	cmt := ""
	for _, c := range comments[0].List {
		cmt += strings.TrimSpace(strings.Replace(c.Text, "//", "", 1)) + "\n"
	}
	cmt = strings.Replace(cmt, "<", "\\<", -1)
	return cmt
}

func writeAPIInfo(astMeta *util.ASTMeta, rpcMeta util.RPCMeta, groups []MethodGroup) error {
	buf := &bytes.Buffer{}

	methNs := rpcMeta.MethodNamespace
	if methNs == "" {
		methNs = "Filecoin"
	}
	tmpl, err := template.New("curl").Parse(`# Sample code of curl

{{ .StartBash }}
# <Inputs> corresponding to the value of Inputs Tag of each API
curl http://<ip>:<port>/rpc/v{{ .APIVersion }} -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer <token>"  -d '{"method": "{{ .Namespace }}.<method>", "params": <Inputs>, "id": 0}'
{{ .EndBash }}
`)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}
	err = tmpl.Execute(buf, map[string]interface{}{
		"APIVersion": rpcMeta.Version,
		"Namespace":  methNs,
		"StartBash":  "```bash",
		"EndBash":    "```",
	})
	if err != nil {
		return fmt.Errorf("exec curl template: %w", err)
	}

	fmt.Fprint(buf, "# Groups\n\n")

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GroupName < groups[j].GroupName
	})
	for _, g := range groups {
		sort.Slice(g.Methods, func(i, j int) bool {
			return g.Methods[i].Name < g.Methods[j].Name
		})

		fmt.Fprintf(buf, "* [%s](#%s)\n", g.GroupName, strings.ToLower(g.GroupName))
		for _, method := range g.Methods {
			fmt.Fprintf(buf, "  * [%s](#%s)\n", method.Name, strings.ToLower(method.Name))
		}
	}

	fmt.Fprintf(buf, "\n")
	for _, g := range groups {
		fmt.Fprintf(buf, "## %s\n\n", g.GroupName)

		for _, m := range g.Methods {
			fmt.Fprintf(buf, "### %s\n", m.Name)
			fmt.Fprintf(buf, "%s\n\n", m.Comment)

			fmt.Fprintf(buf, "Perms: %s\n\n", m.Perm)

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

	return os.WriteFile(filepath.Join(astMeta.Location, "method.md"), buf.Bytes(), 0o644)
}
