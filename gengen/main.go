package main

import (
	"encoding/json"
	flg "flag"
	"fmt"
	"os"
	"time"

	"github.com/filecoin-project/go-filecoin/commands"
	"github.com/filecoin-project/go-filecoin/gengen/util"
	"github.com/filecoin-project/go-filecoin/types"
)

func writeKey(ki *types.KeyInfo, name string, jsonout bool) error {
	addr, err := ki.Address()
	if err != nil {
		return err
	}
	if !jsonout {
		fmt.Fprintf(os.Stderr, "key: %s - %s\n", name, addr.String())                                                          // nolint: errcheck
		fmt.Fprintf(os.Stderr, "run 'go-filecoin wallet import ./%s.key' to add private key for %[1]s to your wallet\n", name) // nolint: errcheck
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
	]
}
$ cat setup.json | gengen > genesis.car

The outputted file can be used by go-filecoin during init to
set the initial genesis block:
$ go-filecoin init --genesisfile=genesis.car
*/

var (
	flag = flg.NewFlagSet(os.Args[0], flg.ExitOnError)
)

func main() {
	var defaultSeed = time.Now().Unix()

	jsonout := flag.Bool("json", false, "sets output to be json")
	testProofsMode := flag.Bool("test-proofs-mode", false, "change sealing, sector packing, PoSt, etc. to be compatible with test environments (overrides proofs mode read from JSON)")
	keypath := flag.String("keypath", ".", "sets location to write key files to")
	outJSON := flag.String("out-json", "", "enables json output and writes it to the given file")
	outCar := flag.String("out-car", "", "writes the generated car file to the give path, instead of stdout")
	configFilePath := flag.String("config", "", "reads configuration from this json file, instead of stdin")
	seed := flag.Int64("seed", defaultSeed, "provides the seed for randomization, defaults to current unix epoch")

	// ExitOnError is set
	flag.Parse(os.Args[1:]) // nolint: errcheck

	jsonEnabled := *jsonout || *outJSON != ""

	cfg, err := readConfig(*configFilePath)
	if err != nil {
		panic(err)
	}

	gengen.ApplyProofsModeDefaults(cfg, !*testProofsMode, isFlagPassed("test-proofs-mode"))

	outfile := os.Stdout
	if *outCar != "" {
		f, err := os.Create(*outCar)
		if err != nil {
			panic(err)
		}
		outfile = f
	}

	info, err := gengen.GenGenesisCar(cfg, outfile, *seed)
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
		fmt.Fprintf(os.Stderr, "created miner %s, owned by %d, power = %s\n", m.Address, m.Owner, m.Power) // nolint: errcheck
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

// isFlagPassed returns true if a flag with the given name was provided by the
// caller.
func isFlagPassed(name string) bool {
	found := false
	flag.Visit(func(f *flg.Flag) {
		if f.Name == name {
			found = true
		}
	})

	return found
}
