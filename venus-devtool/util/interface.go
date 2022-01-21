package util

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

type ASTMeta struct {
	Location string
	*token.FileSet
}

type InterfaceParseOption struct {
	ImportPath string
	IncludeAll bool
	Included   []string
}

type PackageMeta struct {
	Name string
	*ast.Package
}

type FileMeta struct {
	Name string
	*ast.File
}

type InterfaceMeta struct {
	Pkg     PackageMeta
	File    FileMeta
	Name    string
	Defined []InterfaceMethodMeta
	Nested  []string
}

type InterfaceMethodMeta struct {
	Name     string
	Node     ast.Node
	FuncType *ast.FuncType
	Comments []*ast.CommentGroup
}

type ifaceMetaVisitor struct {
	pkg        PackageMeta
	file       FileMeta
	included   map[string]struct{}
	includAll  bool
	comments   ast.CommentMap
	ifaces     []*InterfaceMeta
	ifaceIdxes map[string]int
}

func (iv *ifaceMetaVisitor) Visit(node ast.Node) ast.Visitor {
	st, ok := node.(*ast.TypeSpec)
	if !ok {
		return iv
	}

	iface, ok := st.Type.(*ast.InterfaceType)
	if !ok {
		return iv
	}

	if _, yes := iv.included[st.Name.Name]; !yes && !iv.includAll {
		return iv
	}

	ifaceIdx, ok := iv.ifaceIdxes[st.Name.Name]
	if !ok {
		ifaceIdx = len(iv.ifaces)
		iv.ifaces = append(iv.ifaces, &InterfaceMeta{
			Pkg:  iv.pkg,
			File: iv.file,
			Name: st.Name.Name,
		})
	}

	ifaceMeta := iv.ifaces[ifaceIdx]

	for _, m := range iface.Methods.List {
		switch meth := m.Type.(type) {
		case *ast.Ident:
			ifaceMeta.Nested = append(ifaceMeta.Nested, meth.Name)

		case *ast.FuncType:
			ifaceMeta.Defined = append(ifaceMeta.Defined, InterfaceMethodMeta{
				Name:     m.Names[0].Name,
				Node:     m,
				FuncType: meth,
				Comments: iv.comments.Filter(m).Comments(),
			})
		}
	}

	return iv
}

func ParseInterfaceMetas(opt InterfaceParseOption) ([]*InterfaceMeta, *ASTMeta, error) {
	location, err := FindLocationForImportPath(opt.ImportPath)
	if err != nil {
		return nil, nil, err
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, location, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var metas []*InterfaceMeta

	included := map[string]struct{}{}
	for _, one := range opt.Included {
		included[one] = struct{}{}
	}

	for pname, pkg := range pkgs {
		if strings.HasSuffix(pname, "_test") {
			continue
		}

		visitor := &ifaceMetaVisitor{
			pkg: PackageMeta{
				Name:    pname,
				Package: pkg,
			},
			included:   included,
			includAll:  opt.IncludeAll,
			ifaceIdxes: map[string]int{},
		}

		for fname, file := range pkg.Files {
			visitor.file = FileMeta{
				Name: fname,
				File: file,
			}
			visitor.comments = ast.NewCommentMap(fset, file, file.Comments)
			ast.Walk(visitor, file)
		}

		metas = append(metas, visitor.ifaces...)
	}

	sort.Slice(metas, func(i, j int) bool {
		if metas[i].Pkg != metas[j].Pkg {
			return metas[i].Pkg.Name < metas[j].Pkg.Name
		}

		if metas[i].File != metas[j].File {
			return metas[i].File.Name < metas[j].File.Name
		}

		return metas[i].Name < metas[j].Name
	})

	for mi := range metas {
		sort.Slice(metas[mi].Defined, func(i, j int) bool {
			return metas[mi].Defined[i].Name < metas[mi].Defined[j].Name
		})
	}

	return metas, &ASTMeta{
		Location: location,
		FileSet:  fset,
	}, nil
}
