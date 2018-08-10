package commands

import (
	"io/ioutil"
	"strings"
	"testing"

	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
)

// TODO:
// This code only lives here because the 'NewDaemon' code only exists in this
// package in test code. If it were its own package, we could use it from the
// gengen directory and run tests there

var testConfig = &gengen.GenesisCfg{
	Keys: []string{"bob", "hank", "steve", "laura"},
	PreAlloc: map[string]string{
		"bob":  "10",
		"hank": "50",
	},
	Miners: []gengen.Miner{
		{
			Owner: "bob",
			Power: 5000,
		},
		{
			Owner: "laura",
			Power: 1000,
		},
	},
}

func TestGenGenLoading(t *testing.T) {
	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = gengen.GenGensisCar(testConfig, fi); err != nil {
		t.Fatal(err)
	}

	fi.Close()

	td := NewDaemon(t, func(td *TestDaemon) {
		td.genesisFile = fi.Name()
	}).Start()
	defer td.Shutdown()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	strings.Contains(stdout, `"Power":"5000"`)
	strings.Contains(stdout, `"Power":"1000"`)
}
