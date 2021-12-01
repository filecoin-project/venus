package main

import (
	"bytes"
	"go/ast"
	"go/build"
	"go/format"
	"go/parser"
	"go/token"
	"log"
)

func main() {
	pkg, err := build.Import("github.com/filecoin-project/lotus/api", ".", build.FindOnly)
	if err != nil {
		log.Fatalln(err)
	}

	fset := token.NewFileSet()
	p, err := parser.ParseDir(fset, pkg.Dir, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		log.Fatalln(err)
	}

	for _, fp := range p {
		ast.Inspect(fp, func(n ast.Node) bool {
			decl, ok := n.(*ast.GenDecl)
			if !ok || decl.Tok != token.TYPE || len(decl.Specs) != 1 {
				return true
			}

			spec, ok := decl.Specs[0].(*ast.TypeSpec)
			if !ok {
				return true
			}

			_, ok = spec.Type.(*ast.InterfaceType)
			if !ok {
				return true
			}

			if spec.Name.Name == "FullNode" {
				log.Println("FullNode found")
				var buf bytes.Buffer
				if err := format.Node(&buf, fset, n); err != nil {
					log.Fatalln(err)
				}

				log.Printf("\n%s", buf.String())
			}
			return true
		})
	}
}
