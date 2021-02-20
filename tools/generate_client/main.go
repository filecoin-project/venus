package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"

	"golang.org/x/tools/imports"
)

type Func struct {
	Name   string
	Args   string
	Return string
}

func (f Func) Interface() string {
	return f.Name + f.Args + " " + f.Return
}

func (f Func) Method() string {
	return f.Name + " func" + f.Args + " " + f.Return
}

func main() {
	pkgDir := "app/submodule"
	typeMap := map[string]string{
		"Partition":      "chainApiTypes.Partition",
		"Deadline":       "chainApiTypes.Deadline",
		"MarketDeal":     "chainApiTypes.MarketDeal",
		"BlockTemplate":  "mineApiTypes.BlockTemplate",
		"InvocResult":    "syncApiTypes.InvocResult",
		"ProtocolParams": "chainApiTypes.ProtocolParams",
		"BlockMessage":   "chainApiTypes.BlockMessage",
		"MsgLookup":      "messageApiTypes.MsgLookup",
		"BlockMessages":  "chainApiTypes.BlockMessages",
	}

	_, pkgs, err := collectAPIFile(pkgDir)
	if err != nil {
		log.Fatal(err)
		return
	}

	codes := make(map[string][]Func)
	for _, pkg := range pkgs {
		for _, files := range pkg.Files {
			for _, decl := range files.Decls {
				if funcDecl, ok := decl.(*ast.FuncDecl); ok {
					// just func Decl

					function := Func{}
					receiveName, err := joinFieldList(funcDecl.Recv, typeMap)
					if err != nil {
						log.Fatal(err)
						return
					}
					receiveName = strings.Trim(receiveName, "*")
					if receiveName == "" {
						continue
					}

					function.Name = funcDecl.Name.Name
					if !('A' <= function.Name[0] && function.Name[0] <= 'Z') {
						continue
					}
					// parser parameter
					fieldList, err := joinFieldList(funcDecl.Type.Params, typeMap)
					if err != nil {
						log.Fatal(err)
						return
					}
					function.Args = "(" + fieldList + ")"

					//parser return values
					fieldList, err = joinFieldList(funcDecl.Type.Results, typeMap)
					if err != nil {
						log.Fatal(err)
						return
					}
					if funcDecl.Type.Results != nil && len(funcDecl.Type.Results.List) > 0 {
						function.Return = "(" + fieldList + ")"
					} else {
						function.Return = fieldList
					}

					if _, has := codes[receiveName]; has {
						codes[receiveName] = append(codes[receiveName], function)
					} else {
						codes[receiveName] = []Func{function}
					}
				}
			}
		}
	}

	_ = generateCode(codes, "./app/client/client.go")
}

func generateCode(codelines map[string][]Func, fname string) error {
	fs, err := os.Create(fname)
	if err != nil {
		return err
	}
	packages := `package client
import (
		"context"
		"io"
		"time"
		"github.com/filecoin-project/go-address"
		"github.com/filecoin-project/go-bitfield"
		"github.com/filecoin-project/go-state-types/abi"
		"github.com/filecoin-project/go-state-types/big"
		acrypto "github.com/filecoin-project/go-state-types/crypto"
		"github.com/filecoin-project/go-state-types/dline"
		"github.com/filecoin-project/go-state-types/network"
		"github.com/ipfs/go-cid"
		ipld "github.com/ipfs/go-ipld-format"
		"github.com/libp2p/go-libp2p-core/metrics"
		"github.com/libp2p/go-libp2p-core/peer"
		ma "github.com/multiformats/go-multiaddr"
		chainApiTypes "github.com/filecoin-project/venus/app/submodule/chain"
		mineApiTypes "github.com/filecoin-project/venus/app/submodule/mining"
		"github.com/filecoin-project/venus/app/submodule/mpool"
		syncApiTypes "github.com/filecoin-project/venus/app/submodule/syncer"
		"github.com/filecoin-project/venus/pkg/beacon"
		"github.com/filecoin-project/venus/pkg/chain"
		"github.com/filecoin-project/venus/pkg/chainsync/status"
		"github.com/filecoin-project/venus/pkg/crypto"
		"github.com/filecoin-project/venus/pkg/net"
		"github.com/filecoin-project/venus/pkg/specactors/builtin/miner"
		"github.com/filecoin-project/venus/pkg/types"
		"github.com/filecoin-project/venus/pkg/vm"
		"github.com/filecoin-project/venus/pkg/wallet"
)
`
	builder := strings.Builder{}
	builder.WriteString(packages)

	builder.WriteString("type FullNode struct {\n")
	for _, functions := range codelines {
		for _, function := range functions {
			builder.WriteString("\t" + function.Method() + "\n")
		}
		builder.WriteString("\n")
	}
	builder.WriteString("}\n\n")

	for name, functions := range codelines {
		builder.WriteString("type " + name + " struct {\n")
		for _, function := range functions {
			builder.WriteString("\t" + function.Method() + "\n")
		}
		builder.WriteString("}\n\n")
	}

	_, _ = fs.WriteString(builder.String())
	_ = fs.Close()

	options := &imports.Options{
		TabWidth:  8,
		TabIndent: true,
		Comments:  true,
		Fragment:  true,
	}

	res, err := imports.Process(fname, nil, options)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(fname, res, 0777)
}

func joinFieldList(fields *ast.FieldList, typeMap map[string]string) (string, error) {
	if fields == nil || len(fields.List) == 0 {
		return "", nil
	}
	returnString := ""
	returns := fields.List
	for _, r := range returns {
		tokeString, err := typeString(r.Type, typeMap)
		if err != nil {
			log.Fatal(err)
			return "", err
		}
		returnString += tokeString + ","
	}

	returnString = strings.Trim(returnString, ",")
	return returnString, nil
}

func typeString(token ast.Expr, typeMap map[string]string) (string, error) {
	tokenString := ""
	switch t := token.(type) {
	case *ast.SelectorExpr:
		name, err := typeString(t.X, typeMap)
		if err != nil {
			return tokenString, err
		}
		tokenString += name + "." + t.Sel.String()
	case *ast.Ident:
		if token, has := typeMap[t.String()]; has {
			tokenString += token
		} else {
			tokenString += t.String()
		}

	case *ast.StarExpr:
		name, err := typeString(t.X, typeMap)
		if err != nil {
			return tokenString, err
		}
		tokenString += "*" + name
	case *ast.ArrayType:
		name, err := typeString(t.Elt, typeMap)
		if err != nil {
			return tokenString, err
		}
		tokenString += "[]" + name
	case *ast.InterfaceType:
		tokenString += "interface{}"
	case *ast.ChanType:
		name, err := typeString(t.Value, typeMap)
		if err != nil {
			return tokenString, err
		}
		tokenString += "chan " + name
	case *ast.MapType:
		keyString, err := typeString(t.Key, typeMap)
		if err != nil {
			return tokenString, err
		}
		valueString, err := typeString(t.Value, typeMap)
		if err != nil {
			return tokenString, err
		}
		tokenString += "map[" + keyString + "]" + valueString
	default:
		return "", errors.New("unexpect types")
	}
	return tokenString, nil
}

func collectAPIFile(dir string) (*token.FileSet, map[string]*ast.Package, error) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	fset := token.NewFileSet()
	pkgs := make(map[string]*ast.Package)
	for _, f := range files {
		subDirPath := path.Join(dir, f.Name())

		subModuleFiles, err := ioutil.ReadDir(subDirPath)
		if err != nil {
			return nil, nil, err
		}
		for _, goFile := range subModuleFiles {
			if !goFile.IsDir() && strings.HasSuffix(goFile.Name(), "_api.go") {
				gofileName := path.Join(subDirPath, goFile.Name())
				if src, err := parser.ParseFile(fset, gofileName, nil, 0); err == nil {
					name := src.Name.Name
					pkg, found := pkgs[name]
					if !found {
						pkg = &ast.Package{
							Name:  name,
							Files: make(map[string]*ast.File),
						}
						pkgs[name] = pkg
					}
					pkg.Files[gofileName] = src
				} else {
					return nil, nil, err
				}
			}
		}
	}
	return fset, pkgs, nil
}
