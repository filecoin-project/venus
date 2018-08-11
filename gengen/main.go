package main

import (
	"encoding/json"
	"os"

	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/types"
)

func writeKey(ki *types.KeyInfo, name string) error {
	fi, err := os.Create(name + ".key")
	if err != nil {
		return err
	}
	defer fi.Close()

	return json.NewEncoder(fi).Encode(ki)
}

/* gengen takes as input a json encoded 'Genesis Config'
It outputs a 'car' encoded genesis dag.
For example:
$ cat setup.json
{
	"keys": ["bob", "hank", "steve", "laura"],
	"preAlloc": {
		"bob": "10",
		"hank": "50"
	},
	"miners": [
		{
			"owner":"bob",
			"power": 5000
		},
		{
			"owner": "laura",
			"power": 1000
		}
	]
}
$ cat setup.json | gengen > genesis.car

The outputed file can be used by go-filecoin during init to
set the initial genesis block:
$ go-filecoin init --genesisfile=genesis.car
*/
func main() {
	var cfg gengen.GenesisCfg
	if err := json.NewDecoder(os.Stdin).Decode(&cfg); err != nil {
		panic(err)
	}

	keys, err := gengen.GenGensisCar(&cfg, os.Stdout)
	if err != nil {
		panic(err)
	}

	for name, k := range keys {
		if err := writeKey(k, name); err != nil {
			panic(err)
		}
	}
}
