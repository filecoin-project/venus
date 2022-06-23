package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/venus/venus-devtool/util"
	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
)

func main() {
	app := &cli.App{
		Name:  "bundle-gen",
		Usage: "generate builtin actors for venus-shared",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dst"},
		},
		Action: func(ctx *cli.Context) error {

			metadata, err := builtinactors.ReadEmbeddedBuiltinActorsMetadata()
			if err != nil {
				return err
			}

			buf := &bytes.Buffer{}
			if err := tmpl.Execute(buf, metadata); err != nil {
				return err
			}

			formatted, err := util.FmtFile("", buf.Bytes())
			if err != nil {
				return err
			}

			return ioutil.WriteFile(ctx.String("dst"), formatted, 0744)
		},
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}

var tmpl *template.Template = template.Must(template.New("actor-metadata").Parse(`
// WARNING: This file has automatically been generated
package builtinactors
import (
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)
var EmbeddedBuiltinActorsMetadata []*BuiltinActorsMetadata = []*BuiltinActorsMetadata{
{{- range . }} {
	Network: {{printf "%q" .Network}},
	Version: {{.Version}},
	ManifestCid: types.MustParseCid({{printf "%q" .ManifestCid}}),
	Actors: map[string]cid.Cid {
	{{- range $name, $cid := .Actors }}
		{{printf "%q" $name}}: types.MustParseCid({{printf "%q" $cid}}),
	{{- end }}
	},
},
{{- end -}}
}
`))
