package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/filecoin-project/venus/venus-devtool/util"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:                 "state-type-gen",
		Usage:                "generate types related codes for go-state-types",
		EnableBashCompletion: true,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name: "dst",
			},
		},
		Action: run,
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}

type meta struct {
	opt option

	basePath    string
	includePkgs []string
	skipFiles   []string

	output outputOpt
}

type option struct {
	skipFuncs    map[string]struct{}
	skipStructs  map[string]struct{}
	skipAllVar   bool
	skipVars     []string
	aliasStructs map[string][]struct {
		pkgName string
		newName string
	}
}

type outputOpt struct {
	// 0 所有类型输出到一个文件，1 按文件输出类型
	typ      int
	fileName string
}

var metas = []meta{
	{
		opt:         stateTypesOpt,
		basePath:    "github.com/filecoin-project/go-state-types/builtin",
		includePkgs: getStateTypesIncludePkgs(),
		skipFiles:   []string{"invariants.go", "methods.go"},
		output: outputOpt{
			typ:      0,
			fileName: "state_types_gen.go",
		},
	},
	{
		opt:         sharedTypesOpt,
		basePath:    "github.com/filecoin-project/venus/venus-shared/actors",
		includePkgs: []string{"types"},
		skipFiles:   []string{"_marshal.go", "param.go"},
		output: outputOpt{
			typ: 1,
		},
	},
}

// go-state-types/builtin/
var stateTypesOpt = option{
	skipFuncs: map[string]struct{}{
		"ConstructState": {},
	},
	skipStructs: map[string]struct{}{
		"State":          {},
		"MinerInfo":      {},
		"ConstructState": {},
		"Partition":      {},
		"Deadline":       {},
	},
	skipAllVar: true,
	aliasStructs: map[string][]struct {
		pkgName string
		newName string
	}{
		"WithdrawBalanceParams": {
			{pkgName: "market", newName: "MarketWithdrawBalanceParams"},
			{pkgName: "miner", newName: "MinerWithdrawBalanceParams"},
		},
		"ConstructorParams": {
			{pkgName: "multisig", newName: "MultisigConstructorParams"},
			{pkgName: "paych", newName: "PaychConstructorParams"},
		},
	},
}

// venus-shared/actors/types
var sharedTypesOpt = option{
	skipStructs: map[string]struct{}{
		"ActorV5":     {},
		"TempEthCall": {},
	},
	skipAllVar: false,
	skipVars:   []string{"Eip155ChainID"},
}

var getStateTypesIncludePkgs = func() []string {
	defVersion := actors.Version9
	pkgs := make([]string, 0, 4)
	aliasVesion := map[string]actors.Version{
		"paych": actors.Version8,
	}
	for _, pkg := range []string{"market", "miner", "verifreg", "paych", "multisig", "datacap"} {
		if v, ok := aliasVesion[pkg]; ok {
			pkgs = append(pkgs, fmt.Sprintf("v%v/%s", v, pkg))
		} else {
			pkgs = append(pkgs, fmt.Sprintf("v%v/%s", defVersion, pkg))
		}
	}
	return pkgs
}

func run(cctx *cli.Context) error {
	for _, m := range metas {
		pkgInfos := make([]*pkgInfo, 0, len(m.includePkgs))
		for _, pkg := range m.includePkgs {
			pkgPath := filepath.Join(m.basePath, pkg)
			location, err := util.FindPackageLocation(pkgPath)
			if err != nil {
				return err
			}

			fset := token.NewFileSet()
			pkgs, err := parser.ParseDir(fset, location, filterFile(m.skipFiles), parser.AllErrors|parser.ParseComments)
			if err != nil {
				return err
			}

			for _, pkg := range pkgs {
				pkgInfo := &pkgInfo{name: pkg.Name, path: pkgPath}
				for fileName, file := range pkg.Files {
					visitor := &fileVisitor{
						opt:      m.opt,
						fileInfo: &fileInfo{name: filepath.Base(fileName)},
					}
					ast.Walk(visitor, file)
					visitor.fileInfo.sort()
					pkgInfo.files = append(pkgInfo.files, visitor.fileInfo)
				}
				pkgInfo.sort()
				pkgInfos = append(pkgInfos, pkgInfo)
			}
		}

		sort.Slice(pkgInfos, func(i, j int) bool {
			return pkgInfos[i].name < pkgInfos[j].name
		})

		if err := outputFile(cctx.String("dst"), m.output, pkgInfos, m.opt); err != nil {
			return err
		}
	}

	return nil
}

func filterFile(skipFiles []string) func(fi fs.FileInfo) bool {
	return func(fi fs.FileInfo) bool {
		if strings.Contains(fi.Name(), "cbor_gen.go") {
			return false
		}
		if strings.Contains(fi.Name(), "_test.go") {
			return false
		}
		for _, name := range skipFiles {
			if strings.Contains(fi.Name(), name) {
				return false
			}
		}

		return true
	}
}

type pkgInfo struct {
	name  string
	path  string
	files []*fileInfo
}

func (pi *pkgInfo) sort() {
	sort.Slice(pi.files, func(i, j int) bool {
		return pi.files[i].name < pi.files[j].name
	})
}

type fileVisitor struct {
	opt option
	*fileInfo
}

func (v *fileVisitor) Visit(node ast.Node) (w ast.Visitor) {
	if st, ok := node.(*ast.TypeSpec); ok {
		if !st.Name.IsExported() {
			return v
		}

		name := st.Name.Name
		if _, ok := v.opt.skipStructs[name]; ok {
			return v
		}
		if _, ok := st.Type.(*ast.InterfaceType); ok {
			v.i = append(v.i, name)
			return v
		}

		_, ok = st.Type.(*ast.StructType)
		_, ok2 := st.Type.(*ast.Ident)
		_, ok3 := st.Type.(*ast.ArrayType)
		_, ok4 := st.Type.(*ast.SelectorExpr)
		if ok || ok2 || ok3 || ok4 {
			v.t = append(v.t, name)
		}
	} else if ft, ok := node.(*ast.FuncDecl); ok {
		if !ft.Name.IsExported() || ft.Recv != nil {
			return v
		}

		name := ft.Name.Name
		if _, ok := v.opt.skipFuncs[name]; !ok {
			v.f = append(v.f, name)
		}
	} else if vt, ok := node.(*ast.ValueSpec); ok {
		if !vt.Names[0].IsExported() || len(vt.Names) == 0 {
			return v
		}
		if vt.Names[0].Obj != nil {
			if vt.Names[0].Obj.Kind == ast.Con {
				v.con = append(v.con, vt.Names[0].Name)
			}
			if !v.opt.skipAllVar && vt.Names[0].Obj.Kind == ast.Var {
				if !isContains(v.opt.skipVars, vt.Names[0].Name) {
					v.v = append(v.v, vt.Names[0].Name)
				}
			}
		}
	}

	return v
}

type fileInfo struct {
	name string
	f    []string // function
	t    []string // type
	con  []string // const
	v    []string // var
	i    []string // interface
}

func (fi *fileInfo) sort() {
	sort.Slice(fi.f, func(i, j int) bool {
		return fi.f[i] < fi.f[j]
	})
	sort.Slice(fi.t, func(i, j int) bool {
		return fi.t[i] < fi.t[j]
	})
	sort.Slice(fi.con, func(i, j int) bool {
		return fi.con[i] < fi.con[j]
	})
	sort.Slice(fi.v, func(i, j int) bool {
		return fi.v[i] < fi.v[j]
	})
	sort.Slice(fi.i, func(i, j int) bool {
		return fi.i[i] < fi.i[j]
	})
}

func outputFile(dst string, opt outputOpt, pkgInfos []*pkgInfo, fopt option) error {
	if opt.typ == 0 {
		return writePkgInfo(filepath.Join(dst, opt.fileName), pkgInfos, fopt)
	}

	for _, pi := range pkgInfos {
		for _, fi := range pi.files {
			var fileBuffer bytes.Buffer
			fmt.Fprintf(&fileBuffer, "// Code generated by github.com/filecoin-project/venus/venus-devtool/state-type-gen. DO NOT EDIT.\npackage %s\n\n", "types")

			// write import
			fmt.Fprintln(&fileBuffer, "import (")
			fmt.Fprintf(&fileBuffer, "\"%v\"\n", pi.path)
			fmt.Fprintln(&fileBuffer, ")\n")

			genFileDetail(&fileBuffer, fi, pi.name, fopt)

			formatedBuf, err := util.FmtFile("", fileBuffer.Bytes())
			if err != nil {
				return err
			}

			if err := os.WriteFile(filepath.Join(dst, fi.name), formatedBuf, 0o755); err != nil {
				return err
			}
		}
	}

	return nil
}

func writePkgInfo(dst string, pkgInfos []*pkgInfo, fopt option) error {
	var fileBuffer bytes.Buffer
	fmt.Fprintf(&fileBuffer, "// Code generated by github.com/filecoin-project/venus/venus-devtool/state-type-gen. DO NOT EDIT.\npackage %s\n\n", "types")

	// write import
	fmt.Fprintln(&fileBuffer, "import (")
	for _, info := range pkgInfos {
		fmt.Fprintf(&fileBuffer, "\"%v\"\n", info.path)
	}
	fmt.Fprintln(&fileBuffer, ")\n")

	for _, pi := range pkgInfos {
		merge := fileInfo{}
		for _, fi := range pi.files {
			merge.f = append(merge.f, fi.f...)
			merge.t = append(merge.t, fi.t...)
			merge.con = append(merge.con, fi.con...)
			merge.v = append(merge.v, fi.v...)
		}
		merge.sort()
		fmt.Fprintf(&fileBuffer, "////////// %s //////////\n\n", pi.name)
		genFileDetail(&fileBuffer, &merge, pi.name, fopt)
		fmt.Fprintln(&fileBuffer, "\n")
	}

	formatedBuf, err := util.FmtFile("", fileBuffer.Bytes())
	if err != nil {
		return err
	}

	return os.WriteFile(dst, formatedBuf, 0o755)
}

func genFileDetail(fileBuffer *bytes.Buffer, fi *fileInfo, pkgName string, opt option) {
	genDetail(fileBuffer, fi.con, "const", pkgName, opt)
	genDetail(fileBuffer, fi.v, "var", pkgName, opt)
	genDetail(fileBuffer, fi.i, "type", pkgName, opt)
	genDetail(fileBuffer, fi.t, "type", pkgName, opt)
	genDetail(fileBuffer, fi.f, "var", pkgName, opt)
}

func genDetail(buf *bytes.Buffer, list []string, typ string, pkgName string, opt option) {
	if len(list) == 0 {
		return
	}
	fmt.Fprintf(buf, "%s (\n", typ)
	for _, one := range list {
		if vals, ok := opt.aliasStructs[one]; ok {
			for _, val := range vals {
				if val.pkgName == pkgName {
					fmt.Fprintf(buf, "\t%s = %s.%s\n", val.newName, pkgName, one)
				}
			}
		} else {
			fmt.Fprintf(buf, "\t%s = %s.%s\n", one, pkgName, one)
		}
	}
	fmt.Fprintln(buf, ")")
}

func isContains(list []string, target string) bool {
	for _, v := range list {
		if v == target {
			return true
		}
	}

	return false
}
