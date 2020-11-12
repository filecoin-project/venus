package main

import (
	"encoding/json"
	flg "flag"
	"fmt"
	"os"

	"github.com/filecoin-project/venus/cmd/go-filecoin"
	"github.com/filecoin-project/venus/internal/pkg/crypto"
	"github.com/filecoin-project/venus/tools/gengen/util"
)

func writeKey(ki *crypto.KeyInfo, name string, jsonout bool) error {
	addr, err := ki.Address()
	if err != nil {
		return err
	}
	if !jsonout {
		fmt.Fprintf(os.Stderr, "key: %s - %s\n", name, addr)                                                             // nolint: errcheck
		fmt.Fprintf(os.Stderr, "run 'venus wallet import ./%s.key' to add private key for %[1]s to your wallet\n", name) // nolint: errcheck
	}
	fi, err := os.Create(name + ".key")
	if err != nil {
		return err
	}
	defer fi.Close() // nolint: errcheck

	var wir commands.WalletSerializeResult
	wir.KeyInfo = append(wir.KeyInfo, ki)

	return json.NewEncoder(fi).Encode(wir)
}

/* gengen takes as input a json encoded 'Genesis Config'
It outputs a 'car' encoded genesis dag.
For example:
$ cat setup.json
{
	"keys": 4,
	"preAlloc": [
		"10",
		"50"
	],
	"miners": [
		{
			"owner": 0,
			"power": 5000
		},
		{
			"owner": 1,
			"power": 1000
		}
	],
	Network: "test1",
}
$ cat setup.json | gengen > genesis.car

The outputted file can be used by venus during init to
set the initial genesis block:
$ venus init --genesisfile=genesis.car
*/

var (
	flag = flg.NewFlagSet(os.Args[0], flg.ExitOnError)
)

func main() {
	jsonout := flag.Bool("json", false, "sets output to be json")
	keypath := flag.String("keypath", ".", "sets location to write key files to")
	outJSON := flag.String("out-json", "", "enables json output and writes it to the given file")
	outCar := flag.String("out-car", "", "writes the generated car file to the give path, instead of stdout")
	configFilePath := flag.String("config", "", "reads configuration from this json file, instead of stdin")

	_ = flag.Parse(os.Args[1:])

	jsonEnabled := *jsonout || *outJSON != ""

	cfg, err := readConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	outfile := os.Stdout
	if *outCar != "" {
		f, err := os.Create(*outCar)
		if err != nil {
			panic(err)
		}
		outfile = f
	}

	info, err := gengen.GenGenesisCar(cfg, outfile)
	if err != nil {
		fmt.Println("ERROR", err)
		panic(err)
	}

	for name, k := range info.Keys {
		n := fmt.Sprintf("%s/%d", *keypath, name)
		if err := writeKey(k, n, jsonEnabled); err != nil {
			panic(err)
		}
	}

	if jsonEnabled {
		out, err := json.MarshalIndent(info, "", "  ")
		if err != nil {
			panic(err)
		}

		writer := os.Stderr
		if *outJSON != "" {
			w, err := os.Create(*outJSON)
			if err != nil {
				panic(err)
			}
			writer = w
		}
		_, err = writer.Write(out)
		if err != nil {
			panic(err)
		}
		return
	}

	for _, m := range info.Miners {
		fmt.Fprintf(os.Stderr, "created miner %s, owned by %d, power = %s (raw %s)\n", m.Address, m.Owner, m.QAPower, m.RawPower) // nolint: errcheck
	}
}

func readConfig(filePath string) (*gengen.GenesisCfg, error) {
	configFile := os.Stdin
	if filePath != "" {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		configFile = f
	}

	var cfg gengen.GenesisCfg
	if err := json.NewDecoder(configFile).Decode(&cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %s", err)
	}

	return &cfg, nil
}
