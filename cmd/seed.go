package cmd

import (
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/docker/go-units"
	"github.com/google/uuid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/mitchellh/go-homedir"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/gen"
	"github.com/filecoin-project/venus/pkg/gen/genesis"
	"github.com/filecoin-project/venus/tools/seed"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/miner"
	"github.com/filecoin-project/venus/venus-shared/types"
)

var seedCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Seal sectors for genesis miner.",
	},
	Subcommands: map[string]*cmds.Command{
		"genesis": genesisCmd,

		"pre-seal":            preSealCmd,
		"aggregate-manifests": aggregateManifestsCmd,
	},
}

var genesisCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "manipulate genesis template",
	},
	Subcommands: map[string]*cmds.Command{
		"new":                 genesisNewCmd,
		"add-miner":           genesisAddMinerCmd,
		"add-msis":            genesisAddMsigsCmd,
		"set-vrk":             genesisSetVRKCmd,
		"set-remainder":       genesisSetRemainderCmd,
		"set-network-version": genesisSetActorVersionCmd,
	},
}

var genesisNewCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "create new genesis template",
	},
	Options: []cmds.Option{
		cmds.StringOption("network-name", "network name"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("file", true, true, "The file to write genesis info"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		fileName := req.Arguments[0]
		if fileName == "" {
			return errors.New("seed genesis new [genesis.json]")
		}
		networkName, _ := req.Options["network-name"].(string)
		out := genesis.Template{
			NetworkVersion:   networks.Net2k().Network.GenesisNetworkVersion,
			Accounts:         []genesis.Actor{},
			Miners:           []genesis.Miner{},
			VerifregRootKey:  gen.DefaultVerifregRootkeyActor,
			RemainderAccount: gen.DefaultRemainderAccountActor,
			NetworkName:      networkName,
		}
		if out.NetworkName == "" {
			out.NetworkName = "localnet-" + uuid.New().String()
		}

		genb, err := json.MarshalIndent(&out, "", "  ")
		if err != nil {
			return err
		}

		genf, err := homedir.Expand(fileName)
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, genb, 0644)
	},
}

var genesisAddMinerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "add genesis miner",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("genesis-file", true, true, "genesis file"),
		cmds.StringArg("preseal-file", true, true, "preseal file"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) != 2 {
			return errors.New("seed genesis add-miner [genesis.json] [preseal.json]")
		}

		genf, err := homedir.Expand(req.Arguments[0])
		if err != nil {
			return err
		}

		var template genesis.Template
		genb, err := ioutil.ReadFile(genf)
		if err != nil {
			return fmt.Errorf("read genesis template: %w", err)
		}

		if err := json.Unmarshal(genb, &template); err != nil {
			return fmt.Errorf("unmarshal genesis template: %w", err)
		}

		minf, err := homedir.Expand(req.Arguments[1])
		if err != nil {
			return fmt.Errorf("expand preseal file path: %w", err)
		}
		miners := map[string]genesis.Miner{}
		minb, err := ioutil.ReadFile(minf)
		if err != nil {
			return fmt.Errorf("read preseal file: %w", err)
		}
		if err := json.Unmarshal(minb, &miners); err != nil {
			return fmt.Errorf("unmarshal miner info: %w", err)
		}

		for mn, miner := range miners {
			log.Infof("Adding miner %s to genesis template", mn)
			{
				id := uint64(genesis.MinerStart) + uint64(len(template.Miners))
				maddr, err := address.NewFromString(mn)
				if err != nil {
					return fmt.Errorf("parsing miner address: %w", err)
				}
				mid, err := address.IDFromAddress(maddr)
				if err != nil {
					return fmt.Errorf("getting miner id from address: %w", err)
				}
				if mid != id {
					return fmt.Errorf("tried to set miner t0%d as t0%d", mid, id)
				}
			}

			template.Miners = append(template.Miners, miner)
			log.Infof("Giving %s some initial balance", miner.Owner)
			template.Accounts = append(template.Accounts, genesis.Actor{
				Type:    genesis.TAccount,
				Balance: big.Mul(big.NewInt(50_000_000), big.NewInt(int64(constants.FilecoinPrecision))),
				Meta:    (&genesis.AccountMeta{Owner: miner.Owner}).ActorMeta(),
			})
		}

		genb, err = json.MarshalIndent(&template, "", "  ")
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, genb, 0644)
	},
}

var genesisAddMsigsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("templateFile", true, true, ""),
		cmds.StringArg("csvFile", true, true, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 2 {
			return fmt.Errorf("must specify template file and csv file with accounts")
		}

		genf, err := homedir.Expand(req.Arguments[0])
		if err != nil {
			return err
		}

		csvf, err := homedir.Expand(req.Arguments[1])
		if err != nil {
			return err
		}

		var template genesis.Template
		b, err := ioutil.ReadFile(genf)
		if err != nil {
			return fmt.Errorf("read genesis template: %w", err)
		}

		if err := json.Unmarshal(b, &template); err != nil {
			return fmt.Errorf("unmarshal genesis template: %w", err)
		}

		entries, err := seed.ParseMultisigCsv(csvf)
		if err != nil {
			return fmt.Errorf("parsing multisig csv file: %w", err)
		}

		for i, e := range entries {
			if len(e.Addresses) != e.N {
				return fmt.Errorf("entry %d had mismatch between 'N' and number of addresses", i)
			}

			msig := &genesis.MultisigMeta{
				Signers:         e.Addresses,
				Threshold:       e.M,
				VestingDuration: monthsToBlocks(e.VestingMonths),
				VestingStart:    0,
			}

			template.Accounts = append(template.Accounts, genesis.Actor{
				Type:    genesis.TMultisig,
				Balance: abi.TokenAmount(e.Amount),
				Meta:    msig.ActorMeta(),
			})
		}

		b, err = json.MarshalIndent(&template, "", "  ")
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, b, 0644)
	},
}

func monthsToBlocks(nmonths int) int {
	days := uint64((365 * nmonths) / 12)
	return int(days * 24 * 60 * 60 / constants.MainNetBlockDelaySecs)
}

func parseMultisigCsv(csvf string) ([]seed.GenAccountEntry, error) {
	fileReader, err := os.Open(csvf)
	if err != nil {
		return nil, fmt.Errorf("read multisig csv: %w", err)
	}
	defer fileReader.Close() //nolint:errcheck
	r := csv.NewReader(fileReader)
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read multisig csv: %w", err)
	}
	var entries []seed.GenAccountEntry
	for i, e := range records[1:] {
		var addrs []address.Address
		addrStrs := strings.Split(strings.TrimSpace(e[7]), ":")
		for j, a := range addrStrs {
			addr, err := address.NewFromString(a)
			if err != nil {
				return nil, fmt.Errorf("failed to parse address %d in row %d (%q): %w", j, i, a, err)
			}
			addrs = append(addrs, addr)
		}

		balance, err := types.ParseFIL(strings.TrimSpace(e[2]))
		if err != nil {
			return nil, fmt.Errorf("failed to parse account balance: %w", err)
		}

		vesting, err := strconv.Atoi(strings.TrimSpace(e[3]))
		if err != nil {
			return nil, fmt.Errorf("failed to parse vesting duration for record %d: %w", i, err)
		}

		custodianID, err := strconv.Atoi(strings.TrimSpace(e[4]))
		if err != nil {
			return nil, fmt.Errorf("failed to parse custodianID in record %d: %w", i, err)
		}
		threshold, err := strconv.Atoi(strings.TrimSpace(e[5]))
		if err != nil {
			return nil, fmt.Errorf("failed to parse multisigM in record %d: %w", i, err)
		}
		num, err := strconv.Atoi(strings.TrimSpace(e[6]))
		if err != nil {
			return nil, fmt.Errorf("number of addresses be integer: %w", err)
		}
		if e[0] != "1" {
			return nil, fmt.Errorf("record version must be 1")
		}
		entries = append(entries, seed.GenAccountEntry{
			Version:       1,
			ID:            e[1],
			Amount:        balance,
			CustodianID:   custodianID,
			VestingMonths: vesting,
			M:             threshold,
			N:             num,
			Type:          e[8],
			Sig1:          e[9],
			Sig2:          e[10],
			Addresses:     addrs,
		})
	}

	return entries, nil
}

var genesisSetVRKCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set the verified registry's root key",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("templateFile", true, true, ""),
	},
	Options: []cmds.Option{
		cmds.StringOption("multisig", "CSV file to parse the multisig that will be set as the root key"),
		cmds.StringOption("account", "pubkey address that will be set as the root key (must NOT be declared anywhere else, since it must be given ID 80)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 1 {
			return fmt.Errorf("must specify template file and csv file with accounts")
		}

		genf, err := homedir.Expand(req.Arguments[0])
		if err != nil {
			return err
		}

		var template genesis.Template
		b, err := ioutil.ReadFile(genf)
		if err != nil {
			return fmt.Errorf("read genesis template: %w", err)
		}

		if err := json.Unmarshal(b, &template); err != nil {
			return fmt.Errorf("unmarshal genesis template: %w", err)
		}

		account, _ := req.Options["account"].(string)
		multisig, _ := req.Options["multisig"].(string)
		if len(account) > 0 {
			addr, err := address.NewFromString(account)
			if err != nil {
				return err
			}

			am := genesis.AccountMeta{Owner: addr}

			template.VerifregRootKey = genesis.Actor{
				Type:    genesis.TAccount,
				Balance: big.Zero(),
				Meta:    am.ActorMeta(),
			}
		} else if len(multisig) > 0 {
			csvf, err := homedir.Expand(multisig)
			if err != nil {
				return err
			}

			entries, err := parseMultisigCsv(csvf)
			if err != nil {
				return fmt.Errorf("parsing multisig csv file: %w", err)
			}

			if len(entries) == 0 {
				return fmt.Errorf("no msig entries in csv file: %w", err)
			}

			e := entries[0]
			if len(e.Addresses) != e.N {
				return fmt.Errorf("entry had mismatch between 'N' and number of addresses")
			}

			msig := &genesis.MultisigMeta{
				Signers:         e.Addresses,
				Threshold:       e.M,
				VestingDuration: monthsToBlocks(e.VestingMonths),
				VestingStart:    0,
			}

			act := genesis.Actor{
				Type:    genesis.TMultisig,
				Balance: abi.TokenAmount(e.Amount),
				Meta:    msig.ActorMeta(),
			}

			template.VerifregRootKey = act
		} else {
			return fmt.Errorf("must include either --account or --multisig flag")
		}

		b, err = json.MarshalIndent(&template, "", "  ")
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, b, 0644)
	},
}

var genesisSetRemainderCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set the remainder actor",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("templateFile", true, true, ""),
	},
	Options: []cmds.Option{
		cmds.StringOption("multisig", "CSV file to parse the multisig that will be set as the root key"),
		cmds.StringOption("account", "pubkey address that will be set as the root key (must NOT be declared anywhere else, since it must be given ID 80)"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 1 {
			return fmt.Errorf("must specify template file and csv file with accounts")
		}

		genf, err := homedir.Expand(req.Arguments[0])
		if err != nil {
			return err
		}

		var template genesis.Template
		b, err := ioutil.ReadFile(genf)
		if err != nil {
			return fmt.Errorf("read genesis template: %w", err)
		}

		if err := json.Unmarshal(b, &template); err != nil {
			return fmt.Errorf("unmarshal genesis template: %w", err)
		}

		account, _ := req.Options["account"].(string)
		multisig, _ := req.Options["multisig"].(string)
		if account != "" {
			addr, err := address.NewFromString(account)
			if err != nil {
				return err
			}

			am := genesis.AccountMeta{Owner: addr}

			template.RemainderAccount = genesis.Actor{
				Type:    genesis.TAccount,
				Balance: big.Zero(),
				Meta:    am.ActorMeta(),
			}
		} else if multisig != "" {
			csvf, err := homedir.Expand(multisig)
			if err != nil {
				return err
			}

			entries, err := parseMultisigCsv(csvf)
			if err != nil {
				return fmt.Errorf("parsing multisig csv file: %w", err)
			}

			if len(entries) == 0 {
				return fmt.Errorf("no msig entries in csv file: %w", err)
			}

			e := entries[0]
			if len(e.Addresses) != e.N {
				return fmt.Errorf("entry had mismatch between 'N' and number of addresses")
			}

			msig := &genesis.MultisigMeta{
				Signers:         e.Addresses,
				Threshold:       e.M,
				VestingDuration: monthsToBlocks(e.VestingMonths),
				VestingStart:    0,
			}

			act := genesis.Actor{
				Type:    genesis.TMultisig,
				Balance: abi.TokenAmount(e.Amount),
				Meta:    msig.ActorMeta(),
			}

			template.RemainderAccount = act
		} else {
			return fmt.Errorf("must include either --account or --multisig flag")
		}

		b, err = json.MarshalIndent(&template, "", "  ")
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, b, 0644)
	},
}

var genesisSetActorVersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Set the version that this network will start from",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("genesisFile", true, true, ""),
		cmds.StringArg("actorVersion", true, true, ""),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) < 2 {
			return fmt.Errorf("must specify genesis file and network version (e.g. '0'")
		}

		genf, err := homedir.Expand(req.Arguments[0])
		if err != nil {
			return err
		}

		var template genesis.Template
		b, err := ioutil.ReadFile(genf)
		if err != nil {
			return fmt.Errorf("read genesis template: %w", err)
		}

		if err := json.Unmarshal(b, &template); err != nil {
			return fmt.Errorf("unmarshal genesis template: %w", err)
		}

		nv, err := strconv.ParseUint(req.Arguments[1], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing network version: %w", err)
		}

		if nv > uint64(constants.NewestNetworkVersion) {
			return fmt.Errorf("invalid network version: %d", nv)
		}

		template.NetworkVersion = network.Version(nv)

		b, err = json.MarshalIndent(&template, "", "  ")
		if err != nil {
			return err
		}

		return ioutil.WriteFile(genf, b, 0644)
	},
}

var preSealCmd = &cmds.Command{
	Options: []cmds.Option{
		cmds.StringOption("sector-dir", "sector directory").WithDefault("~/.genesis-sectors"),
		cmds.StringOption("miner-addr", "specify the future address of your miner").WithDefault("t01000"),
		cmds.StringOption("sector-size", "specify size of sectors to pre-seal").WithDefault("2KiB"),
		cmds.StringOption("ticket-preimage", "set the ticket preimage for sealing randomness").WithDefault("venus is fire"),
		cmds.IntOption("num-sectors", "select number of sectors to pre-seal").WithDefault(int(1)),
		cmds.IntOption("sector-offset", "how many sector ids to skip when starting to seal").WithDefault(int(0)),
		cmds.StringOption("key", "(optional) Key to use for signing / owner/worker addresses").WithDefault(""),
		cmds.BoolOption("fake-sectors", "").WithDefault(false),
		cmds.IntOption("network-version", "specify network version").WithDefault(int(networks.Net2k().Network.GenesisNetworkVersion)),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		sdir, _ := req.Options["sector-dir"].(string)
		sbroot, err := homedir.Expand(sdir)
		if err != nil {
			return err
		}

		addr, _ := req.Options["miner-addr"].(string)
		maddr, err := address.NewFromString(addr)
		if err != nil {
			return err
		}

		var ki *crypto.KeyInfo
		if key, _ := req.Options["key"].(string); key != "" {
			ki = new(crypto.KeyInfo)
			kh, err := ioutil.ReadFile(key)
			if err != nil {
				return err
			}
			kb, err := hex.DecodeString(string(kh))
			if err != nil {
				return err
			}
			if err := json.Unmarshal(kb, ki); err != nil {
				return err
			}
		}

		ssize, _ := req.Options["sector-size"].(string)
		sectorSizeInt, err := units.RAMInBytes(ssize)
		if err != nil {
			return err
		}
		sectorSize := abi.SectorSize(sectorSizeInt)

		nv := networks.Net2k().Network.GenesisNetworkVersion
		ver, _ := req.Options["network-version"].(int)
		if ver >= 0 {
			nv = network.Version(ver)
		}

		spt, err := miner.SealProofTypeFromSectorSize(sectorSize, nv)
		if err != nil {
			return err
		}

		sectorOffset, _ := req.Options["sector-offset"].(int)
		numSectors, _ := req.Options["num-sectors"].(int)
		ticketPreimage, _ := req.Options["ticket-preimage"].(string)
		fakeSectors, _ := req.Options["fake-sectors"].(bool)
		gm, key, err := seed.PreSeal(maddr, spt, abi.SectorNumber(uint64(sectorOffset)), numSectors, sbroot, []byte(ticketPreimage), ki, fakeSectors)
		if err != nil {
			return err
		}

		return seed.WriteGenesisMiner(maddr, sbroot, gm, key)
	},
}

var aggregateManifestsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "aggregate a set of preseal manifests into a single file",
	},
	Options: []cmds.Option{
		cmds.StringsOption("file", "file path"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		var inputs []map[string]genesis.Miner
		files, _ := req.Options["file"].([]string)
		for _, infi := range files {
			fi, err := os.Open(infi)
			if err != nil {
				return err
			}
			var val map[string]genesis.Miner
			if err := json.NewDecoder(fi).Decode(&val); err != nil {
				return err
			}

			inputs = append(inputs, val)
			if err := fi.Close(); err != nil {
				return err
			}
		}

		output := make(map[string]genesis.Miner)
		for _, in := range inputs {
			for maddr, val := range in {
				if gm, ok := output[maddr]; ok {
					tmp, err := mergeGenMiners(gm, val)
					if err != nil {
						return err
					}
					output[maddr] = tmp
				} else {
					output[maddr] = val
				}
			}
		}

		blob, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return err
		}

		return re.Emit(string(blob))
	},
}

func mergeGenMiners(a, b genesis.Miner) (genesis.Miner, error) {
	if a.SectorSize != b.SectorSize {
		return genesis.Miner{}, fmt.Errorf("sector sizes mismatch, %d != %d", a.SectorSize, b.SectorSize)
	}

	return genesis.Miner{
		Owner:         a.Owner,
		Worker:        a.Worker,
		PeerID:        a.PeerID,
		MarketBalance: big.Zero(),
		PowerBalance:  big.Zero(),
		SectorSize:    a.SectorSize,
		Sectors:       append(a.Sectors, b.Sectors...),
	}, nil
}
