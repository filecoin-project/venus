package util

import (
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

type InterfaceMeta struct {
	Pkg      string
	File     string
	Name     string
	Defined  []InterfaceMethodMeta
	Included []string
}

type InterfaceMethodMeta struct {
	Name     string
	Node     ast.Node
	FuncType *ast.FuncType
	Comments []*ast.CommentGroup
}

type ifaceMetaVisitor struct {
	pname      string
	fname      string
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

	ifaceIdx, ok := iv.ifaceIdxes[st.Name.Name]
	if !ok {
		ifaceIdx = len(iv.ifaces)
		iv.ifaces = append(iv.ifaces, &InterfaceMeta{
			Pkg:  iv.pname,
			File: iv.fname,
			Name: st.Name.Name,
		})
	}

	ifaceMeta := iv.ifaces[ifaceIdx]

	for _, m := range iface.Methods.List {
		switch meth := m.Type.(type) {
		case *ast.Ident:
			ifaceMeta.Included = append(ifaceMeta.Included, meth.Name)

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

func ParseInterfaceMetas(importPath string) ([]*InterfaceMeta, error) {
	location, err := FindLocationForImportPath(importPath)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, location, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var metas []*InterfaceMeta

	for pname, pkg := range pkgs {
		if strings.HasSuffix(pname, "_test") {
			continue
		}

		visitor := &ifaceMetaVisitor{
			pname:      pname,
			ifaceIdxes: map[string]int{},
		}

		for fname, file := range pkg.Files {
			visitor.fname = fname
			visitor.comments = ast.NewCommentMap(fset, file, file.Comments)
			ast.Walk(visitor, file)
		}

		metas = append(metas, visitor.ifaces...)
	}

	sort.Slice(metas, func(i, j int) bool {
		if metas[i].Pkg != metas[j].Pkg {
			return metas[i].Pkg < metas[j].Pkg
		}

		if metas[i].File != metas[j].File {
			return metas[i].File < metas[j].File
		}

		return metas[i].Name < metas[j].Name
	})

	for mi := range metas {
		sort.Slice(metas[mi].Defined, func(i, j int) bool {
			return metas[mi].Defined[i].Name < metas[mi].Defined[j].Name
		})
	}

	return metas, nil
}
