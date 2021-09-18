package main

import (
	"bytes"
	"fmt"
	"github.com/filecoin-project/venus/pkg/types/specactors"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	"golang.org/x/xerrors"
)

var actors = map[string][]int{
	"account":  specactors.Versions,
	"cron":     specactors.Versions,
	"init":     specactors.Versions,
	"market":   specactors.Versions,
	"miner":    specactors.Versions,
	"multisig": specactors.Versions,
	"paych":    specactors.Versions,
	"power":    specactors.Versions,
	"system":   specactors.Versions,
	"reward":   specactors.Versions,
	"verifreg": specactors.Versions,
}

func main() {
	if err := generateAdapters(); err != nil {
		fmt.Println(err)
		return
	}

	if err := generatePolicy("pkg/types/specactors/policy/policy.go"); err != nil {
		fmt.Println(err)
		return
	}

	if err := generateBuiltin("pkg/types/specactors/builtin/builtin.go"); err != nil {
		fmt.Println(err)
		return
	}
}

func generateAdapters() error {
	for act, versions := range actors {
		actDir := filepath.Join("pkg/types/specactors/builtin", act)

		if err := generateState(actDir); err != nil {
			return err
		}

		if err := generateMessages(actDir); err != nil {
			return err
		}

		{
			af, err := ioutil.ReadFile(filepath.Join(actDir, "actor.go.template"))
			if err != nil {
				return xerrors.Errorf("loading actor template: %w", err)
			}

			tpl := template.Must(template.New("").Funcs(template.FuncMap{
				"import": func(v int) string { return getVersionImports()[v] },
			}).Parse(string(af)))

			var b bytes.Buffer

			err = tpl.Execute(&b, map[string]interface{}{
				"versions":      versions,
				"latestVersion": specactors.LatestVersion,
			})
			if err != nil {
				return err
			}

			if err := ioutil.WriteFile(filepath.Join(actDir, fmt.Sprintf("%s.go", act)), b.Bytes(), 0666); err != nil {
				return err
			}
		}
	}

	return nil
}

func generateState(actDir string) error {
	af, err := ioutil.ReadFile(filepath.Join(actDir, "state.go.template"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip
		}

		return xerrors.Errorf("loading state adapter template: %w", err)
	}

	for _, version := range specactors.Versions {
		tpl := template.Must(template.New("").Funcs(template.FuncMap{}).Parse(string(af)))

		var b bytes.Buffer

		err := tpl.Execute(&b, map[string]interface{}{
			"v":      version,
			"import": getVersionImports()[version],
		})
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(actDir, fmt.Sprintf("v%d.go", version)), b.Bytes(), 0666); err != nil {
			return err
		}
	}

	return nil
}

func generateMessages(actDir string) error {
	af, err := ioutil.ReadFile(filepath.Join(actDir, "message.go.template"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip
		}

		return xerrors.Errorf("loading message adapter template: %w", err)
	}

	for _, version := range specactors.Versions {
		tpl := template.Must(template.New("").Funcs(template.FuncMap{}).Parse(string(af)))

		var b bytes.Buffer

		err := tpl.Execute(&b, map[string]interface{}{
			"v":      version,
			"import": getVersionImports()[version],
		})
		if err != nil {
			return err
		}

		if err := ioutil.WriteFile(filepath.Join(actDir, fmt.Sprintf("message%d.go", version)), b.Bytes(), 0666); err != nil {
			return err
		}
	}

	return nil
}

func generatePolicy(policyPath string) error {

	pf, err := ioutil.ReadFile(policyPath + ".template")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip
		}

		return xerrors.Errorf("loading policy template file: %w", err)
	}

	tpl := template.Must(template.New("").Funcs(template.FuncMap{
		"import": func(v int) string { return getVersionImports()[v] },
	}).Parse(string(pf)))
	var b bytes.Buffer

	err = tpl.Execute(&b, map[string]interface{}{
		"versions":      specactors.Versions,
		"latestVersion": specactors.LatestVersion,
	})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(policyPath, b.Bytes(), 0666); err != nil {
		return err
	}

	return nil
}

func generateBuiltin(builtinPath string) error {

	bf, err := ioutil.ReadFile(builtinPath + ".template")
	if err != nil {
		if os.IsNotExist(err) {
			return nil // skip
		}

		return xerrors.Errorf("loading builtin template file: %w", err)
	}

	tpl := template.Must(template.New("").Funcs(template.FuncMap{
		"import": func(v int) string { return getVersionImports()[v] },
	}).Parse(string(bf)))
	var b bytes.Buffer

	err = tpl.Execute(&b, map[string]interface{}{
		"versions":      specactors.Versions,
		"latestVersion": specactors.LatestVersion,
	})
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(builtinPath, b.Bytes(), 0666); err != nil {
		return err
	}

	return nil
}

func getVersionImports() map[int]string {
	versionImports := make(map[int]string, specactors.LatestVersion)
	for _, v := range specactors.Versions {
		if v == 0 {
			versionImports[v] = "/"
		} else {
			versionImports[v] = "/v" + strconv.Itoa(v) + "/"
		}
	}

	return versionImports
}
