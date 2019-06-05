# localnet

localnet is a FAST binary tool that quickly and easily, sets up a local network
on the users computer. The network will stay standing till the program is closed.

### Example

```
localnet -shell
```

### Building

The localnet tool expects that you can already build `go-filecoin`. Please refer
to the README in the root of this project for details.

localnet is only compatible with `go-filecoin` binaries built from the same git ref.

```
go build -o localnet main.go
```

### Usage

```
Usage of ./localnet:
  -binpath go-filecoin
    	set the binary used when executing go-filecoin commands
  -blocktime duration
    	duration for blocktime (default 5s)
  -miner-collateral string
    	amount of fil each miner will use for collateral (default "500")
  -miner-count int
    	number of miners (default 5)
  -miner-expiry string
    	expiry value used when creating ask for miners (default "86400")
  -miner-price string
    	price value used when creating ask for miners (default "0.0000000010")
  -shell
    	setup a filecoin client node and enter into a shell ready to use
  -small-sectors
    	enables small sectors (default true)
  -workdir string
    	set the working directory used to store filecoin repos
```

### Addional notes from the author

The default settings are pretty close to what the devnets run. The tool defaults
to small sectors, but that can be changed by passing `-small-sectors=false`. To
make it a bit easier to use, there is also a `-shell` flag that can be passed
which will drop the user into a shell with a go-filecoin daemon already running
and ready to be used with `go-filecoin`.

_Note: Using regular sized sectors with localnet can be incredibly taxing on a
system and should probably be avoided on laptops due to the number of miners
running. The overall miner count can be reduced from the default `5` by passing
the `-miner-count` flag._

```
localnet $ ./localnet -small-sectors=false -shell
```

_I ran `./localnet -small-sectors=false -miner-count=2` on my laptop ( i7-8550U
CPU @ 1.80GHz / 16GB Ram) and it took just under 40 minutes, the equivalent with
small sectors took 2 minutes._

A few helpful things to note when working with localnet
1. All nodes and filecoin repositories will be cleaned up if the program exits.
   The program will not exit till it receives a `SIGTERM` (ctrl-c)
2. Every command that is ran will be printed to the output, along with which node
   ran it. The nodes repository is also printed first.
   - `08:51:48.014  INFO /tmp/local: RunCmd: /tmp/localnet417209521/0 [go-filecoin ...`
3. The stdout and stderr are written to disk under the repository directory
   - **stderr** `/tmp/localnet417209521/0/daemon.stderr`
   - **stdout** `/tmp/localnet417209521/0/daemon.stdout`
4. The localnet tool will copy the `go-filecoin` binary specifed by `binpath` and
   place it in a `bin` directory under each nodes repository which is used to execute
   all commands. To ensure binary compatibility, it's best to execute this same binary
5. You can run commands against any of the nodes by using the `-repodir` flag with
   the go-filecoin binary
   - `/tmp/localnet417209521/0/bin/go-filecoin -repodir=/tmp/localnet417209521/0 id`
    
