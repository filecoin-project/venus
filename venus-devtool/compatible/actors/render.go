package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/filecoin-project/lotus/chain/actors"
	"github.com/filecoin-project/venus/venus-devtool/util"
)

func importPath(v int) string {
	if v == 0 {
		return "/"
	}

	return fmt.Sprintf("/v%d/", v)
}

func render(tpath string, versions []int) error {
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
		err = renderSeparated(t, dir, versions)
	} else {
		err = renderSingle(t, dir, versions)
	}

	if err != nil {
		return err
	}

	return nil
}

func renderSingle(t *template.Template, dir string, versions []int) error {
	var buf bytes.Buffer
	err := t.Execute(&buf, map[string]interface{}{
		"versions":      versions,
		"latestVersion": actors.LatestVersion,
	})
	if err != nil {
		return fmt.Errorf("render single template: %w", err)
	}

	formatted, err := util.FmtFile("", buf.Bytes())
	if err != nil {
		return fmt.Errorf("format go source file : %w", err)
	}

	err = os.WriteFile(filepath.Join(dir, t.Name()+".go"), formatted, 0o644)
	if err != nil {
		return fmt.Errorf("write to file: %w", err)
	}

	return nil
}

func renderSeparated(t *template.Template, dir string, versions []int) error {
	var buf bytes.Buffer
	for _, v := range versions {
		buf.Reset()

		err := t.Execute(&buf, map[string]interface{}{
			"v":             v,
			"import":        importPath(v),
			"latestVersion": actors.LatestVersion,
		})
		if err != nil {
			return fmt.Errorf("render separated template for ver %d: %w", v, err)
		}

		data, err := removeDuplicateLinesFromBytes(buf.Bytes(), "builtin16")
		if err != nil {
			return fmt.Errorf("remove duplicate lines failed: %v", err)
		}

		formatted, err := util.FmtFile("", data)
		if err != nil {
			return fmt.Errorf("format go source file for ver %d: %w", v, err)
		}

		err = os.WriteFile(filepath.Join(dir, fmt.Sprintf("%s.v%d.go", t.Name(), v)), formatted, 0o644)
		if err != nil {
			return fmt.Errorf("write to file for ver %d: %w", v, err)
		}
	}

	return nil
}

func removeDuplicateLinesFromBytes(data []byte, dumpString string) ([]byte, error) {
	seenLines := make(map[string]struct{})
	var orderedLines []string

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, dumpString) {
			if _, exists := seenLines[line]; !exists {
				seenLines[line] = struct{}{}
				orderedLines = append(orderedLines, line)
			}
		} else {
			orderedLines = append(orderedLines, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanner failed: %v", err)
	}

	var result bytes.Buffer
	for _, line := range orderedLines {
		result.WriteString(line + "\n")
	}

	return result.Bytes(), nil
}
