package gengen_test

import (
	"io/ioutil"
	"strings"
	"testing"

	. "github.com/filecoin-project/go-filecoin/gengen/util"
	th "github.com/filecoin-project/go-filecoin/testhelpers"
)

var testConfig = &GenesisCfg{
	Keys: []string{"bob", "hank", "steve", "laura"},
	PreAlloc: map[string]string{
		"bob":  "10",
		"hank": "50",
	},
	Miners: []Miner{
		{
			Owner: "bob",
			Power: 50,
		},
		{
			Owner: "laura",
			Power: 10,
		},
	},
}

func TestGenGenLoading(t *testing.T) {
	fi, err := ioutil.TempFile("", "gengentest")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = GenGenesisCar(testConfig, fi); err != nil {
		t.Fatal(err)
	}

	_ = fi.Close()

	td := th.NewDaemon(t, th.GenesisFile(fi.Name())).Start()
	defer td.Shutdown()

	o := td.Run("actor", "ls").AssertSuccess()

	stdout := o.ReadStdout()
	strings.Contains(stdout, `"Power":"50"`)
	strings.Contains(stdout, `"Power":"10"`)
}
