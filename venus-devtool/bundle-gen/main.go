package main

import (
	"fmt"
	"os"
	"text/template"

	"github.com/urfave/cli/v2"

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

			fi, err := os.Create(ctx.String("dst"))
			if err != nil {
				return err
			}
			defer fi.Close() //nolint

			return tmpl.Execute(fi, metadata)
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
