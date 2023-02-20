package main

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/urfave/cli/v2"

	"github.com/filecoin-project/venus/venus-devtool/util"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

func splitOverride(override string) (string, string) {
	x := strings.Split(override, "=")
	return x[0], x[1]
}

func main() {
	app := &cli.App{
		Name:  "bundle-gen",
		Usage: "generate builtin actors for venus-shared",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "dst"},
			&cli.StringFlag{Name: "version"},
			&cli.StringFlag{Name: "release"},
			&cli.StringSliceFlag{Name: "release_overrides"},
		},
		Action: func(ctx *cli.Context) error {
			// read metadata from the embedded bundle, includes all info except git tags
			metadata, err := actors.ReadEmbeddedBuiltinActorsMetadata()
			if err != nil {
				return err
			}

			// IF args have been provided, extract git tag info from them, otherwise
			// rely on previously embedded metadata for git tags.
			if ver := ctx.String("version"); len(ver) != 0 {
				// overrides are in the format network_name=override
				gitTag := ctx.String("release")
				packedActorsVersion, err := strconv.Atoi(ver[1:])
				if err != nil {
					return err
				}

				overrides := map[string]string{}
				for _, override := range ctx.StringSlice("release_overrides") {
					k, v := splitOverride(override)
					overrides[k] = v
				}
				for _, m := range metadata {
					if int(m.Version) == packedActorsVersion {
						override, ok := overrides[m.Network]
						if ok {
							m.BundleGitTag = override
						} else {
							m.BundleGitTag = gitTag
						}
					} else {
						m.BundleGitTag = getOldGitTagFromEmbeddedMetadata(m)
					}
				}
			}

			buf := &bytes.Buffer{}
			if err := tmpl.Execute(buf, metadata); err != nil {
				return err
			}

			formatted, err := util.FmtFile("", buf.Bytes())
			if err != nil {
				return err
			}

			return os.WriteFile(ctx.String("dst"), formatted, 0o744)
		},
	}

	app.Setup()

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "ERR: %v\n", err) // nolint: errcheck
	}
}

func getOldGitTagFromEmbeddedMetadata(m *actors.BuiltinActorsMetadata) string {
	for _, v := range actors.EmbeddedBuiltinActorsMetadata {
		// if we agree on the manifestCid for the previously embedded metadata, use the previously set tag
		if m.Version == v.Version && m.Network == v.Network && m.ManifestCid == v.ManifestCid {
			return m.BundleGitTag
		}
	}

	return ""
}

var tmpl *template.Template = template.Must(template.New("actor-metadata").Parse(`
// WARNING: This file has automatically been generated
package actors
import (
	"github.com/ipfs/go-cid"
)
var EmbeddedBuiltinActorsMetadata []*BuiltinActorsMetadata = []*BuiltinActorsMetadata{
{{- range . }} {
	Network: {{printf "%q" .Network}},
	Version: {{.Version}},
	{{if .BundleGitTag}} BundleGitTag: {{printf "%q" .BundleGitTag}}, {{end}}
	ManifestCid: mustParseCid({{printf "%q" .ManifestCid}}),
	Actors: map[string]cid.Cid {
	{{- range $name, $cid := .Actors }}
		{{printf "%q" $name}}: mustParseCid({{printf "%q" $cid}}),
	{{- end }}
	},
},
{{- end -}}
}

func mustParseCid(c string) cid.Cid {
	ret, err := cid.Decode(c)
	if err != nil {
		panic(err)
	}

	return ret
}
`))
