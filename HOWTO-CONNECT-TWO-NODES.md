# HOW TO: Connect Two Nodes

## Node 1

Initialize go-filecoin in the default directory, and use the genesis file from go-filecoin source.  These files will now be in $HOME/.filecoin

```go-filecoin init --genesisfile ./fixtures/genesis.car```

Get the address of Node 1 with `go-filecoin id`:
```
$ go-filecoin id
    {
	"Addresses": [
		"/ip4/127.0.0.1/tcp/6000/ipfs/QmVk7A2vEBFr9GyKyQ3wvDmTWj8M4H3jubHUDc3CktdoXL",
		"/ip4/172.16.200.201/tcp/6000/ipfs/QmVk7A2vEBFr9GyKyQ3wvDmTWj8M4H3jubHUDc3CktdoXL"
	],
	"ID": "QmVk7A2vEBFr9GyKyQ3wvDmTWj8M4H3jubHUDc3CktdoXL",
	"AgentVersion": "",
	"ProtocolVersion": "",
	"PublicKey": ""
    }
$ export NODE1_ADDR=/ip4/127.0.0.1/tcp/6000/ipfs/QmVk7A2vEBFr9GyKyQ3wvDmTWj8M4H3jubHUDc3CktdoXL    
```

Run the daemon with `go-filecoin daemon`

## Node 2
This assumes you wish to run two nodes on the same machine for development/testing purposes. If you are trying to connect two separate machines then you will not need to use `--repodir` everywhere unless you do not want to use the default filecoin directory in $HOME/.filecoin.

In another terminal, choose where you want the other FileCoin RepoDir to be, and supply this to the intialization script. You must always specify `--repodir` if it isn't in the default location.
Then specify non-default values for the api.address and swarm.address. NOTE ANY VALUE HAS TO BE SINGLE AND DOUBLE-QUOTED.

```
export FCRD=$HOME/.filecoin2
go-filecoin init --genesisfile ./fixtures/genesis.car --repodir=$FCRD
go-filecoin config api.address '"/ip4/127.0.0.1/tcp/3456"' \
    --repodir=$FCRD
go-filecoin config swarm.address '"/ip4/0.0.0.0/tcp/6002"' \
    --repodir=$FCRD
go-filecoin daemon --repodir=$FCRD
```

Get the address of Node 2
```
$ go-filecoin id --repodir=$FCRD
    {
   	"Addresses": [
   		"/ip4/127.0.0.1/tcp/6001/ipfs/QmXcUJ7YoFQEY7w8bpxuFvQtY9VHUkYfx6AZW6Bi2MDFbs",
   		"/ip4/172.16.200.201/tcp/6001/ipfs/QmXcUJ7YoFQEY7w8bpxuFvQtY9VHUkYfx6AZW6Bi2MDFbs"
   	],
   	"ID": "QmXcUJ7YoFQEY7w8bpxuFvQtY9VHUkYfx6AZW6Bi2MDFbs",
   	"AgentVersion": "",
   	"ProtocolVersion": "",
   	"PublicKey": ""
   }
$ export NODE2_ADDR=/ip4/127.0.0.1/tcp/6001/ipfs/QmXcUJ7YoFQEY7w8bpxuFvQtY9VHUkYfx6AZW6Bi2MDFbs    
```

Now connect Node 2 to Node 1 using the address retrieved for Node 1:
```
go-filecoin swarm connect $NODE1_ADDR --repodir=$FCRD
```

Connect Node 1 to Node 2:
```
go-filecoin swarm connect $NODE2_ADDR
```

You should be able to see who connected peers are:
```
# Peers of node 1
go-filecoin swarm peers
/ip4/127.0.0.1/tcp/6001/ipfs/QmXcUJ7YoFQEY7w8bpxuFvQtY9VHUkYfx6AZW6Bi2MDFbs

# Peers of node 2
go-filecoin swarm peers --repodir=$FCRD
/ip4/127.0.0.1/tcp/6000/ipfs/QmVk7A2vEBFr9GyKyQ3wvDmTWj8M4H3jubHUDc3CktdoXL
```