# gengen

gengen is a filecoin tool that builds the genesis.car, which is used to seed the
genesis block.

### Example

```
go-filecon $ ./tools/gengen/gengen --keypath fixtures --out-car fixtures/genesis.car --out-json fixtures/gen.json --config fixtures/setup.json
```

### Building

The gengen tool expects that you can already build `go-filecoin`. Please refer
to the README in the root of this project for details.

```
go build -o gengen main.go
```

### Usage

```
Usage of ./tools/gengen:
  -config string
    	reads configuration from this json file, instead of stdin
  -json
    	sets output to be json
  -keypath string
    	sets location to write key files to (default ".")
  -out-car string
    	writes the generated car file to the give path, instead of stdout
  -out-json string
    	enables json output and writes it to the given file
  -seed int
    	provides the seed for randomization, defaults to current unix epoch (default 1553189402)
  -test-proofs-mode boolean
       configures sealing and PoSt generation to be less computationally expensive
```

#### Configuration File

The gengen configuration file defines the artifacts produced.

- `keys` defines the number of keys which will be produced
- `preAlloc` is an array defining the amount of FIL for each key
- `miners` is an array defining miners, the `owner` is the key index, and `power` is the amount of power the miner will have in the genesis block.

Example

```json
{
  "keys": 5,
  "preAlloc": [
    "1000000000000",
    "1000000000000",
    "1000000000000",
    "1000000000000",
    "1000000000000"
  ],
  "miners": [{
    "owner": 0,
    "power": 1
  }]
}
```
