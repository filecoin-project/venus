To set up a node and connect into an existing cluster:
```
rm -fr ~/.filecoin
# filecoin requires the same genesis block as the cluster and you should
# also give your node a name so you can easily find it on the dashboard.
# The easiest way to connect into the labweek cluster is using the
# following flags (you could of course address update the config directly
# if you wanted instead):
go-filecoin init --genesisfile ./fixtures/genesis.car --cluster-teamweek
go-filecoin config stats.nickname '"<yournodename>"'
go-filecoin daemon
# In a new terminal, tell your local node to stream logs to the aggregator:
go-filecoin log streamto <aggregator multiaddr>
# Ex: go-filecoin log streamto /dns4/test.kittyhawk.wtf/tcp/19000

# Dashboard: http://test.kittyhawk.wtf:8010
# Dashboard aggregator collection (TCP): test.kittyhawk.wtf:19000 
# Block explorer: http://test.kittyhawk.wtf:8000
# Faucet: http://test.kittyhawk.wtf:9797
# SSH (dev team only please):
#   ssh -i <AWS - Terraform SSH key from 1Password> ubuntu@test.kittyhawk.wtf
#   sudo -s
#   docker ps
#   docker logs -f <container_name> # filecoin-{0,4} 
#   # see processes running in container
#   docker top filecoin-{0,4}
```
