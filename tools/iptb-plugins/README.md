# How to use IPTB with venus

These scripts allow one to:

- Create the IPTB testbed
- Initialize the testbed nodes with a genesis file
- Start the testbed nodes
- Configure the testbed nodes wallet addresses and miner address
- Connect the testbed nodes together

## Setup
First, ensure you have the latest version of IPTB installed:
```shell
$> go get -u github.com/ipfs/iptb
```

Next, ensure you have venus **installed**, IPTB requires that the venus bin be in your path:
```shell
$> cd $GOPATH/src/github.com/filecoin-project/venus
$> go run build/main.go deps
$> go run build/main.go install
```

Now, build the `localfilecoin` iptb plugin:
```shell
$> make iptb
```
And verify the plugin was created:
```shell
$> ls $HOME/testbed/plugins/
localfilecoin.so
```

*NOTE:* If you want to create Docker nodes, be sure to build the docker image first:
```shell
$> docker build .
```

## Initialization

### Simple
Create 10 local Filecoin nodes:
```shell
sh tools/iptb-plugins/filecoin/local/scripts/prepMining.sh 10
```

Create 10 Docker Filecoin nodes:
```shell
sh tools/iptb-plugins/filecoin/docker/scripts/prepMining.sh 10
```

### Advanced

Create a 10 node `testbed`:
```shell
iptb testbed create --count 10 --type localfilecoin
```
Verify the testbed was created:
```shell
$> ls $HOME/testbed/testbeds/
default/
$> ls $HOME/testbed/testbeds/default/
0/  1/  2/  3/  4/  5/  6/  7/  8/  9/  nodespec.json
$> cat $HOME/testbed/testbeds/default/nodespec.json
[
  {
    "Type": "localfilecoin",
    "Dir": "/home/frrist/testbed/testbeds/default/0",
    "Attrs": {}
  },
...
  {
    "Type": "localfilecoin",
    "Dir": "/home/frrist/testbed/testbeds/default/9",
    "Attrs": {}
  }
]
```
NOTE: multiple testbeds can exist under the `$HOME/testbed/testbeds` directory, they may be created & interacted with by using the `--testbed` flag.

Initialize the nodes in testbed `default`:
```shell
$> iptb init
node[0] exit 0

initializing filecoin node at /home/frrist/testbed/testbeds/default/0

...

node[9] exit 0

initializing filecoin node at /home/frrist/testbed/testbeds/default/9
```
Verify the nodes initialized their repositories correctly:
```shell
$> ls $HOME/testbed/testbeds/default/0/
badger/  config.toml  keystore/  snapshots/  version  wallet/
```
NOTE: arguments can be passed to nodes with any command by adding them after the `--` argument, e.g.:
```shell
$> iptb init -- --genesisfile=/some/path/to/it --devnet-nightly
```

Start the testbed nodes:
```shell
$> iptb start
INFO-[/home/frrist/testbed/testbeds/default/9]   Started daemon: /home/frrist/testbed/testbeds/default/9, pid: 6843
open /home/frrist/testbed/testbeds/default/9/api: no such file or directory
...
```

Connect all nodes together (ignore the errors about self dials, that is expected):
```shell
$> iptb connect
```

Verify the connections were made:
```shell
$> iptb run -- venus swarm peers
node[0] exit 0

/ip4/127.0.0.1/tcp/33427/ipfs/QmVihFTmJDpWc8iAQXcbp4mavc6dWDuHqktm9EfFyTvBiC
/ip4/127.0.0.1/tcp/36005/ipfs/QmSLPRjGzqoYVJcmKSgsAJWgtSGCemYKKndrpTjRtpXr4d
/ip4/127.0.0.1/tcp/36893/ipfs/QmNrCUhm9Hgp9sQz6MsZcAWX6j83eJzD4SHvmEzA8Xfh76
/ip4/127.0.0.1/tcp/37705/ipfs/QmPcMfAGupZa5kfzB8FF4YeetrY5r7vDLeak9hC2FBB8aW
/ip4/127.0.0.1/tcp/39583/ipfs/QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu
/ip4/127.0.0.1/tcp/40291/ipfs/QmYvTR6L8MpU6NNJgaBHvfG6DnVwL2kmox5VWDmYp2ipX2
/ip4/127.0.0.1/tcp/40663/ipfs/QmSsTBuiG7N3z2utNNTc4p6N6mTbKgWbqiSW2h9HiRKq7M
/ip4/127.0.0.1/tcp/40671/ipfs/QmTG8s5TMUfGA1hwZ9umMBjG2bRD9DJTQg14U3XzLpqR23
/ip4/127.0.0.1/tcp/45755/ipfs/QmTqjCLwJhxG4LyKRKhd536bDJ9UEBDix4HNUvRJMK8qL2
```

## Running Commands

Run a command on all the nodes:
```
$> iptb run -- venus wallet addrs ls
node[0] exit 0

fcqd8399qra4a94tspmplcrh68x7vkhqzxaxtk6nw

...

node[9] exit 0

fcqn9054lff4s9v6rlt76h08k4ra0gt9xmpymcl9w
```

Or just the even number nodes:
```shell
$> iptb run [0,2,4,6,8] -- venus id
node[0] exit 0

{
        "Addresses": [
                "/ip4/127.0.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/192.168.0.116/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/172.17.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/172.18.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8"
        ],
        "ID": "Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
        "AgentVersion": "",
        "ProtocolVersion": "",
        "PublicKey": ""
}
...
```
Or nodes 3-5:
```shell
$> iptb run [3-5] -- venus swarm peers
node[3] exit 0

node[4] exit 0

node[5] exit 0
```

Jump into a shell for a node:
```shell
$> iptb shell 0
$> venus id
{
        "Addresses": [
                "/ip4/127.0.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/192.168.0.116/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/172.17.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
                "/ip4/172.18.0.1/tcp/44311/ipfs/Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8"
        ],
        "ID": "Qmbb5hawLiz1md6hcAiW98p1SLSE4u1cV5BNZB5VnhKQQ8",
        "AgentVersion": "",
        "ProtocolVersion": "",
        "PublicKey": ""
}
$> exit
$> iptb shell 1
$> venus id
{
        "Addresses": [
                "/ip4/127.0.0.1/tcp/39583/ipfs/QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu",
                "/ip4/192.168.0.116/tcp/39583/ipfs/QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu",
                "/ip4/172.17.0.1/tcp/39583/ipfs/QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu",
                "/ip4/172.18.0.1/tcp/39583/ipfs/QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu"
        ],
        "ID": "QmaTJHZeSTorvtCSstk1LJs5HvHUt7vmbpaJuaYJFWtWiu",
        "AgentVersion": "",
        "ProtocolVersion": "",
        "PublicKey": ""
}
```

Happy Coding
