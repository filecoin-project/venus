package main

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/filecoin-project/lotus/chain/actors"
)

func importPath(v int) string {
	if v == 0 {
		return "/"
	}

	return fmt.Sprintf("/v%d/", v)
}

func render(tpath string) error {
	dir := filepath.Dir(tpath)
	fname := filepath.Base(tpath)

	data, err := os.ReadFile(tpath)
	if err != nil {
		return fmt.Errorf("read file content: %w", err)
	}

	var tname string
	separated := false
	if strings.HasSuffix(fname, separatedGoTemplateExt) {
		tname = fname[:len(fname)-separatedGoTemplateExtLen]
		separated = true
	} else {
		tname = fname[:len(fname)-goTemplateExtLen]
	}

	funcMap := template.FuncMap{}
	if !separated {
		funcMap["import"] = importPath
	}

	t, err := template.New(tname).Funcs(funcMap).Parse(string(data))
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	if separated {
		err = renderSeparated(t, dir)
	} else {
		err = renderSingle(t, dir)
	}

	if err != nil {
		return err
	}

	return nil
}

func renderSingle(t *template.Template, dir string) error {
	var buf bytes.Buffer
	err := t.Execute(&buf, map[string]interface{}{
		"versions":      actors.Versions,
		"latestVersion": actors.LatestVersion,
	})

	if err != nil {
		return fmt.Errorf("render single template: %w", err)
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return fmt.Errorf("format go source file: %w", err)
	}

	err = os.WriteFile(filepath.Join(dir, t.Name()+".go"), formatted, 0644)
	if err != nil {
		return fmt.Errorf("write to file: %w", err)
	}

	return nil
}

func renderSeparated(t *template.Template, dir string) error {
	var buf bytes.Buffer
	for _, v := range actors.Versions {
		buf.Reset()

		err := t.Execute(&buf, map[string]interface{}{
			"v":      v,
			"import": importPath(v),
		})

		if err != nil {
			return fmt.Errorf("render separated template for ver %d: %w", v, err)
		}

		formatted, err := format.Source(buf.Bytes())
		if err != nil {
			return fmt.Errorf("format go source file for ver %d: %w", v, err)
		}

		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s.v%d.go", t.Name(), v)), formatted, 0644)
		if err != nil {
			return fmt.Errorf("write to file for ver %d: %w", v, err)
		}
	}

	return nil
}
