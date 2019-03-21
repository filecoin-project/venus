# gengen

gengen is a filecoin tool that builds the genesis.car, which is used to seed the
genesis block.

### Example

```
go-filecon $ ./gengen/gengen --keypath fixtures --out-car fixtures/genesis.car --out-json fixtures/gen.json --config fixtures/setup.json
```

### Building

The gengen tool expects that you can already build `go-filecoin`. Please refer
to the README in the root of this project for details.

```
go build -o gengen main.go
```

### Usage

```
Usage of ./gengen:
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
```
