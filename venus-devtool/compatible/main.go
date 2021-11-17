package main

import (
	"fmt"
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"log"
	"path/filepath"

	_ "github.com/filecoin-project/lotus/build"
)

func main() {
	pkg, err := build.Import("github.com/filecoin-project/lotus/gen", ".", build.FindOnly)
	if err != nil {
		log.Fatalln("find pkg", err)
	}

	targetFile := filepath.Join(pkg.Dir, "main.go")
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, targetFile, nil, 0)
	if err != nil {
		log.Fatalln("parse file", err)
	}

	ast.Inspect(f, func(n ast.Node) bool {
		if expr, ok := n.(*ast.CallExpr); ok {
			if fn, ok := expr.Fun.(*ast.SelectorExpr); ok {
				if fn.Sel != nil && fn.Sel.Name == "WriteMapEncodersToFile" {
					ci, err := parseGenCallExpr(expr)
					if err != nil {
						log.Fatalln(err)
					}

					fmt.Printf("%s: %s\n", ci.pkgName, filepath.Dir(filepath.Join("github.com/filecoin-project/lotus", ci.path)))
					for ti := range ci.typeNames {
						fmt.Printf("\t%s\n", ci.typeNames[ti])
					}

					fmt.Println("")
				}
			}
		}
		return true
	})
}

type callInfo struct {
	path      string
	pkgName   string
	typeNames []string
}

func parseGenCallExpr(expr *ast.CallExpr) (*callInfo, error) {
	if numIn := len(expr.Args); numIn < 3 {
		return nil, fmt.Errorf("not enough args, got %d", numIn)
	}

	first, ok := expr.Args[0].(*ast.BasicLit)
	if !ok || first.Kind != token.STRING {
		return nil, fmt.Errorf("1st arg should be a string, got %T", first)
	}

	second, ok := expr.Args[1].(*ast.BasicLit)
	if !ok || second.Kind != token.STRING {
		return nil, fmt.Errorf("2nd arg should be a string, got %T", second)
	}

	typeNames := make([]string, 0, len(expr.Args)-2)

	for _, typArg := range expr.Args[2:] {
		lit, ok := typArg.(*ast.CompositeLit)
		if !ok {
			return nil, fmt.Errorf("should be CompositeLit, got %T", typArg)
		}

		sel, ok := lit.Type.(*ast.SelectorExpr)
		if !ok || sel.Sel == nil {
			return nil, fmt.Errorf("unexpected literal type: %T", sel)
		}

		typeNames = append(typeNames, sel.Sel.Name)
	}

	return &callInfo{
		path:      first.Value[1 : len(first.Value)-1],
		pkgName:   second.Value[1 : len(second.Value)-1],
		typeNames: typeNames,
	}, nil
}
