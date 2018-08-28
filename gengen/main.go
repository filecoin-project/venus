package main

import (
	"encoding/json"
	"fmt"
	"os"

	gengen "github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/types"
)

func writeKey(ki *types.KeyInfo, name string) error {
	addr, err := ki.Address()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "key: %s - %s\n", name, addr.String())                                                          // nolint: errcheck
	fmt.Fprintf(os.Stderr, "run 'go-filecoin wallet import ./%s.key' to add private key for %[1]s to your wallet\n", name) // nolint: errcheck
	fi, err := os.Create(name + ".key")
	if err != nil {
		return err
	}
	defer fi.Close() // nolint: errcheck

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

The outputted file can be used by go-filecoin during init to
set the initial genesis block:
$ go-filecoin init --genesisfile=genesis.car
*/
func main() {
	var cfg gengen.GenesisCfg
	if err := json.NewDecoder(os.Stdin).Decode(&cfg); err != nil {
		panic(err)
	}

	info, err := gengen.GenGenesisCar(&cfg, os.Stdout)
	if err != nil {
		panic(err)
	}

	for _, m := range info.Miners {
		fmt.Fprintf(os.Stderr, "created miner %s, owned by %s, power = %d\n", m.Address, m.Owner, m.Power) // nolint: errcheck
	}

	for name, k := range info.Keys {
		if err := writeKey(k, name); err != nil {
			panic(err)
		}
	}
}
