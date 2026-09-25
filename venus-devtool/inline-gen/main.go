package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

const (
	stateGlobal = iota
	stateTemplate
	stateGen
)

var data = map[string]interface{}{}

func main() {
	db, err := os.ReadFile(os.Args[2])
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
	}
	if err := json.Unmarshal(db, &data); err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	err = filepath.Walk(os.Args[1], processFile)
	if err != nil {
		log.Fatalf("Error walking directory: %v", err)
	}
}

func processFile(path string, info os.FileInfo, err error) error {
	if err != nil {
		return err
	}
	if info.IsDir() {
		return nil
	}
	if filepath.Ext(path) != ".go" {
		return nil
	}

	fileBytes, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(fileBytes), "\n")

	outLines, templateLines, insertAt, err := processLines(lines)
	if err != nil {
		log.Printf("Error processing file %s: %v", path, err)
		return nil
	}

	if len(templateLines) > 0 {
		tpl, err := template.New("").Funcs(template.FuncMap{
			"import": func(v float64) string {
				if v == 0 {
					return "/"
				}
				return fmt.Sprintf("/v%d/", int(v))
			},
			"add": func(a, b float64) float64 {
				return a + b
			},
		}).Parse(strings.Join(templateLines, "\n"))
		if err != nil {
			return fmt.Errorf("parsing template: %v", err)
		}

		var b bytes.Buffer
		err = tpl.Execute(&b, data)
		if err != nil {
			return fmt.Errorf("executing template: %v", err)
		}

		rendered := strings.Split(b.String(), "\n")
		out := make([]string, 0, len(outLines)+len(rendered))
		out = append(out, outLines[:insertAt]...)
		out = append(out, rendered...)
		out = append(out, outLines[insertAt:]...)
		outLines = out
	}

	out, err := format.Source([]byte(strings.Join(outLines, "\n")))
	if err != nil {
		return fmt.Errorf("formatting %s: %v", path, err)
	}
	if !bytes.Equal(out, fileBytes) {
		err = os.WriteFile(path, out, 0)
		if err != nil {
			return fmt.Errorf("writing file: %v", err)
		}
	}
	return nil
}

func processLines(lines []string) ([]string, []string, int, error) {
	outLines := make([]string, 0, len(lines))
	templateLines := make([]string, 0)
	insertAt := 0
	state := stateGlobal

	for _, line := range lines {
		switch state {
		case stateGlobal:
			outLines = append(outLines, line)
			if strings.TrimSpace(line) == `/* inline-gen template` {
				state = stateTemplate
			}
		case stateTemplate:
			outLines = append(outLines, line)
			if strings.TrimSpace(line) == `/* inline-gen start */` {
				state = stateGen
				insertAt = len(outLines)
				continue
			}
			templateLines = append(templateLines, line)
		case stateGen:
			if strings.TrimSpace(line) != `/* inline-gen end */` {
				continue
			}
			state = stateGlobal
			templateLines = append(templateLines, line)
		}
	}
	if state != stateGlobal {
		return nil, nil, 0, fmt.Errorf("unexpected end of file while in state %d", state)
	}

	return outLines, templateLines, insertAt, nil
}
