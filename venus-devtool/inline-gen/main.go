package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/filecoin-project/venus/venus-devtool/util"
)

const (
	stateGlobal = iota
	stateTemplate
	stateGen
)

func main() {
    db, err := ioutil.ReadFile(os.Args[2])
    if err != nil {
        log.Fatalf("Error reading file: %v", err)
    }
    var data map[string]interface{}
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

    fileBytes, err := ioutil.ReadFile(path)
    if err != nil {
        return err
    }

    lines := strings.Split(string(fileBytes), "\n")

    outLines, templateLines, err := processLines(lines)
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

        outLines = append(outLines, strings.Split(b.String(), "\n")...)
    }

    if len(outLines) != len(lines) {
        err = ioutil.WriteFile(path, []byte(strings.Join(outLines, "\n")), 0)
        if err != nil {
            return fmt.Errorf("writing file: %v", err)
        }
    }
    return nil
}

func processLines(lines []string) ([]string, []string, error) {
    outLines := make([]string, 0, len(lines))
    templateLines := make([]string, 0)
    state := stateGlobal

    for i, line := range lines {
        switch state {
        case stateGlobal:
            outLines = append(outLines, line)
            if strings.TrimSpace(line) == `/* inline-gen template` {
                state = stateTemplate
            }
		case stateTemplate:
			outLines = append(outLines, line)
			if strings.TrimSpace(line) == /* inline-gen start */ {
				state = stateGen
continue
}
templateLines = append(templateLines, line)
		case stateGen:
			if strings.TrimSpace(line) != /* inline-gen end */ {
		continue
		}
		state = stateGlobal
		templateLines = nil
			}
		}
if state != stateGlobal {
    return nil, nil, fmt.Errorf("unexpected end of file while in state %d", state)
}
return outLines, templateLines, nil
}