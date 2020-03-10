## Generating genesis-sectors

Genesis sectors are generated using the [Lotus](https://docs.lotu.sh/en+setup-local-dev-net) seed tool.
Follow the directions there to install lotus and then run:
```
./lotus-seed pre-seal --sector-size 2048 --num-sectors 2 --miner-addr=t0106
```
The sector-size must be one of the sizes in use by the proof system. And the miner address should be the address
of the bootstrap miner to be created with gengen. By default in gengen this address starts at t0106 and the id address number is incremented for every additional miner.
This will create a `.genesis-sectors` directory in your home directory.

### Configure setup.json for gengen

The genesis setup.json file will need to be updated to match the data in the presealed directory.

1. Unencode the preseal key using `xxd -r -p ~/.genesis-sectors/pre-seal-t0106.key` and copy the private key to the importKeys.
2. Use the information provided in `pre-seal-t0106.json` to update the sector size, commR, commD, sectorNum, commP, pieceSize, and endEpoch of the sectors in setup.json.

### Initialize with presealed sectors

To initialize a bootstrap miner using presealed sectors:
```bash
./go-filecoin init --genesisfile=genesis.car --wallet-keyfile=[miner key]--miner-actor-address=t0106 --presealed-sectordir=[preseal directory]
```
Where `miner key` is the key file imported for the miner. This will be the keyfile whose name is the `minerOwner` specified in setup.json 
(e.g. if setup.json has `keysToGen` set to 5, then the miner owner key file will be `fixtures/test/5.key).`
The presealed sector directory should be set to `[path to home directory]/.genesis-sectors` by default. 