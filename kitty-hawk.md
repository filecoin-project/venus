To set up a node and connect into an existing cluster:
```
rm -fr ~/.filecoin
# filecoin must be initialized in the right way to connect to the existing
# cluster.  You must init with the same genesis.car file as the bootstrappers.
# Finally you must configure your daemon with the proper bootstrap peers.
# For lab week the easiest way to do this is by calling init with the
# cluster-teamweek flag set.  Alternatively you can set the config's bootstrap
# addrs manually after running init and before running daemon.
go-filecoin init --genesisfile ./fixtures/genesis.car --cluster-teamweek
go-filecoin daemon
# In a new terminal, tell your local node to stream logs to the aggregator:
go-filecoin log streamto <aggregator multiaddr>
# Ex: go-filecoin log streamto /dns4/cluster.kittyhawk.wtf/tcp/19000

# Dashboard: http://test.kittyhawk.wtf:8010
# Dashboard aggregator collection (TCP): test.kittyhawk.wtf:19000 
# Block explorer: http://test.kittyhawk.wtf:8000
# Faucet: http://test.kittyhawk.wtf:9797
```
