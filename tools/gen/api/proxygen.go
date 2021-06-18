package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unicode"

	"github.com/filecoin-project/venus/app/client/funcrule"
	"github.com/ipfs/go-path"

	"golang.org/x/xerrors"
)

// Rule[perm:read,ignore:true]
var rulePattern = `Rule\[(?P<rule>.*)\]`

type ruleKey = string

const (
	rkPerm   ruleKey = "perm"
	rkIgnore ruleKey = "ignore"
)

var defaultPerm = []string{"perm", "read"}

var regRule, _ = regexp.Compile(rulePattern)

func parseRule(comment string) (*funcrule.Rule, map[string][]string) {
	rule := new(funcrule.Rule)
	match := regRule.FindStringSubmatch(comment)
	tags := map[string][]string{}
	if len(match) == 2 {
		pairs := strings.Split(match[1], ",")
		for _, v := range pairs {
			pair := strings.Split(v, ":")
			if len(pair) != 2 {
				continue
			}
			switch pair[0] {
			case rkPerm:
				tags[rkPerm] = pair
				rule.Perm = pair[1]
			case rkIgnore:
				ig, err := strconv.ParseBool(pair[1])
				if err != nil {
					panic("the rule tag is invalid format")
				}
				rule.Ignore = ig
			}
		}
	} else {
		rule.Perm = "read"
		tags[rkPerm] = defaultPerm
	}
	return rule, tags
}

type methodMeta struct {
	node  ast.Node
	ftype *ast.FuncType
}

type Visitor struct {
	Methods map[string]map[string]*methodMeta
	Include map[string][]string
}

func (v *Visitor) Visit(node ast.Node) ast.Visitor {
	st, ok := node.(*ast.TypeSpec)
	if !ok {
		return v
	}

	iface, ok := st.Type.(*ast.InterfaceType)
	if !ok {
		return v
	}
	if v.Methods[st.Name.Name] == nil {
		v.Methods[st.Name.Name] = map[string]*methodMeta{}
	}
	for _, m := range iface.Methods.List {
		switch ft := m.Type.(type) {
		case *ast.Ident:
			v.Include[st.Name.Name] = append(v.Include[st.Name.Name], ft.Name)
		case *ast.FuncType:
			v.Methods[st.Name.Name][m.Names[0].Name] = &methodMeta{
				node:  m,
				ftype: ft,
			}
		}
	}

	return v
}

func main() {
	if err := generate("./app/submodule", "apiface", "client", "./app/client/full.go"); err != nil {
		fmt.Println("error: ", err)
	}
}

func typeName(e ast.Expr, pkg string) (string, error) {
	switch t := e.(type) {
	case *ast.SelectorExpr:
		return t.X.(*ast.Ident).Name + "." + t.Sel.Name, nil
	case *ast.Ident:
		pstr := t.Name
		if !unicode.IsLower(rune(pstr[0])) && pkg != "client" {
			pstr = "client." + pstr // todo src pkg name
		}
		return pstr, nil
	case *ast.ArrayType:
		subt, err := typeName(t.Elt, pkg)
		if err != nil {
			return "", err
		}
		return "[]" + subt, nil
	case *ast.StarExpr:
		subt, err := typeName(t.X, pkg)
		if err != nil {
			return "", err
		}
		return "*" + subt, nil
	case *ast.MapType:
		k, err := typeName(t.Key, pkg)
		if err != nil {
			return "", err
		}
		v, err := typeName(t.Value, pkg)
		if err != nil {
			return "", err
		}
		return "map[" + k + "]" + v, nil
	case *ast.StructType:
		if len(t.Fields.List) != 0 {
			return "", xerrors.Errorf("can't struct")
		}
		return "struct{}", nil
	case *ast.InterfaceType:
		if len(t.Methods.List) != 0 {
			return "", xerrors.Errorf("can't interface")
		}
		return "interface{}", nil
	case *ast.ChanType:
		subt, err := typeName(t.Value, pkg)
		if err != nil {
			return "", err
		}
		if t.Dir == ast.SEND {
			subt = "->chan " + subt
		} else {
			subt = "<-chan " + subt
		}
		return subt, nil
	default:
		return "", xerrors.Errorf("unknown type")
	}
}

// nolint
func isGoFile(fi os.FileInfo) bool {
	name := fi.Name()
	return !fi.IsDir() &&
		len(name) > 0 && name[0] != '.' && // ignore .files
		filepath.Ext(name) == ".go"
}

func generate(rootPath string, pkg, outpkg, outfile string) error {
	fset := token.NewFileSet()
	apiDir, err := filepath.Abs(rootPath)
	if err != nil {
		return err
	}
	outfile, err = filepath.Abs(outfile)
	if err != nil {
		return err
	}

	type methodInfo struct {
		Name                                     string
		node                                     ast.Node
		Tags                                     map[string][]string
		NamedParams, ParamNames, Results, DefRes string
	}
	type strinfo struct {
		Name    string
		Methods map[string]*methodInfo
		Include []string
	}

	type meta struct {
		Infos   map[string]*strinfo
		Imports map[string]string
		OutPkg  string
	}
	visitor := &Visitor{make(map[string]map[string]*methodMeta), map[string][]string{}}
	m := &meta{
		OutPkg:  outpkg,
		Infos:   map[string]*strinfo{},
		Imports: map[string]string{},
	}
	//filter := isGoFile
	pkgs, err := parser.ParseDir(fset, path.Join([]string{apiDir, pkg}), nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return err
	}
	ap := pkgs[pkg]

	ast.Walk(visitor, ap)
	ignoreMethods := map[string][]string{}
	for _, f := range ap.Files {
		cmap := ast.NewCommentMap(fset, f, f.Comments)
		for _, im := range f.Imports {
			m.Imports[im.Path.Value] = im.Path.Value
			if im.Name != nil {
				m.Imports[im.Path.Value] = im.Name.Name + " " + m.Imports[im.Path.Value]
			}
		}

		for ifname, methods := range visitor.Methods {
			if _, ok := m.Infos[ifname]; !ok {
				m.Infos[ifname] = &strinfo{
					Name:    ifname,
					Methods: map[string]*methodInfo{},
					Include: visitor.Include[ifname],
				}
			}
			info := m.Infos[ifname]
			for mname, node := range methods {
				filteredComments := cmap.Filter(node.node).Comments()
				if _, ok := info.Methods[mname]; !ok {
					var params, pnames []string
					for _, param := range node.ftype.Params.List {
						pstr, err := typeName(param.Type, outpkg)
						if err != nil {
							return err
						}

						c := len(param.Names)
						if c == 0 {
							c = 1
						}

						for i := 0; i < c; i++ {
							pname := fmt.Sprintf("p%d", len(params))
							pnames = append(pnames, pname)
							params = append(params, pname+" "+pstr)
						}
					}

					var results []string
					for _, result := range node.ftype.Results.List {
						rs, err := typeName(result.Type, outpkg)
						if err != nil {
							return err
						}
						results = append(results, rs)
					}

					defRes := ""
					if len(results) > 1 {
						defRes = results[0]
						switch {
						case defRes[0] == '*' || defRes[0] == '<', defRes == "interface{}":
							defRes = "nil"
						case defRes == "bool":
							defRes = "false"
						case defRes == "string":
							defRes = `""`
						case defRes == "int", defRes == "int64", defRes == "uint64", defRes == "uint":
							defRes = "0"
						default:
							defRes = "*new(" + defRes + ")"
						}
						defRes += ", "
					}

					info.Methods[mname] = &methodInfo{
						Name:        mname,
						node:        node.node,
						Tags:        map[string][]string{},
						NamedParams: strings.Join(params, ", "),
						ParamNames:  strings.Join(pnames, ", "),
						Results:     strings.Join(results, ", "),
						DefRes:      defRes,
					}

				}

				// try to parse tag info
				if len(filteredComments) > 0 {
					cmt := filteredComments[0].List[0].Text
					rule, tags := parseRule(cmt)
					info.Methods[mname].Tags[rkPerm] = tags[rkPerm]
					// remove ignore method
					if rule.Ignore {
						ignoreMethods[ifname] = append(ignoreMethods[ifname], mname)
					}
				}
			}
		}
	}
	for ifname, mnames := range ignoreMethods {
		for _, mname := range mnames {
			delete(m.Infos[ifname].Methods, mname)
		}
	}
	w, err := os.OpenFile(outfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	err = doTemplate(w, m, `// Code generated by github.com/filecoin-project/tools/gen/api. DO NOT EDIT.

package {{.OutPkg}}

import (
{{range .Imports}}	{{.}}
{{end}}
)
`)
	if err != nil {
		return err
	}

	err = doTemplate(w, m, `
{{range .Infos}}
type {{.Name}}Struct struct {
{{range .Include}}	{{.}}Struct
{{end}}{{range .Methods}}	{{.Name}} func({{.NamedParams}}) ({{.Results}}) `+"`"+`{{range .Tags}}{{index . 0}}:"{{index . 1}}"{{end}}`+"`"+`
{{end}}}
{{end}}

`)
	return err
}

func doTemplate(w io.Writer, info interface{}, templ string) error {
	t := template.Must(template.New("").
		Funcs(template.FuncMap{}).Parse(templ))

	return t.Execute(w, info)
}
