package genesis

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/filecoin-project/venus/pkg/consensusfault"
	"github.com/filecoin-project/venus/pkg/fork"
	"github.com/filecoin-project/venus/pkg/fvm"
	"github.com/filecoin-project/venus/pkg/util/ffiwrapper/impl"
	"github.com/filecoin-project/venus/pkg/vmsupport"
	builtinactors "github.com/filecoin-project/venus/venus-shared/builtin-actors"
	"github.com/filecoin-project/venus/venus-shared/types"

	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/vm/gas"

	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/state/tree"

	builtin0 "github.com/filecoin-project/specs-actors/actors/builtin"
	verifreg0 "github.com/filecoin-project/specs-actors/actors/builtin/verifreg"
	adt0 "github.com/filecoin-project/specs-actors/actors/util/adt"

	"github.com/filecoin-project/go-state-types/network"

	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/multisig"

	"github.com/filecoin-project/venus/venus-shared/actors/adt"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/account"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/verifreg"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/market"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/power"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/cron"

	init_ "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/actors/builtin/reward"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin/system"

	"github.com/filecoin-project/venus/venus-shared/actors/builtin"

	"github.com/ipfs/go-cid"
	cbor "github.com/ipfs/go-ipld-cbor"
	logging "github.com/ipfs/go-log/v2"

	"github.com/filecoin-project/go-address"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/crypto"

	"github.com/filecoin-project/venus/pkg/chain"
	sigs "github.com/filecoin-project/venus/pkg/crypto"
	"github.com/filecoin-project/venus/pkg/repo"
	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/pkg/vm"
)

const AccountStart = 100
const MinerStart = 1000
const MaxAccounts = MinerStart - AccountStart

var log = logging.Logger("genesis")

type GenesisBootstrap struct { //nolint
	Genesis *types.BlockHeader
}

/*
From a list of parameters, create a genesis block / initial state

The process:
- Bootstrap state (MakeInitialStateTree)
  - Create empty state
  - Create system actor
  - Make init actor
    - Create accounts mappings
    - Set NextID to MinerStart
  - Setup Reward (1.4B fil)
  - Setup Cron
  - Create empty power actor
  - Create empty market
  - Create verified registry
  - Setup burnt fund address
  - Initialize account / msig balances
- Instantiate early vm with genesis syscalls
  - Create miners
    - Each:
      - power.CreateMiner, set msg value to PowerBalance
      - market.AddFunds with correct value
      - market.PublishDeals for related sectors
    - Set network power in the power actor to what we'll have after genesis creation
	- Recreate reward actor state with the right power
    - For each precommitted sector
      - Get deal weight
      - Calculate QA Power
      - Remove fake power from the power actor
      - Calculate pledge
      - Precommit
      - Confirm valid

Data Types:

PreSeal :{
  CommR    CID
  CommD    CID
  SectorID SectorNumber
  Deal     market.DealProposal # Start at 0, self-deal!
}

Genesis: {
	Accounts: [ # non-miner, non-singleton actors, max len = MaxAccounts
		{
			Type: "account" / "multisig",
			Value: "attofil",
			[Meta: {msig settings, account key..}]
		},...
	],
	Miners: [
		{
			Owner, Worker Addr # ID
			MarketBalance, PowerBalance TokenAmount
			SectorSize uint64
			PreSeals []PreSeal
		},...
	],
}

*/

func MakeInitialStateTree(ctx context.Context, bs bstore.Blockstore, template Template) (*tree.State, map[address.Address]address.Address, error) {
	// Create empty state tree

	cst := cbor.NewCborStore(bs)
	_, err := cst.Put(context.TODO(), []struct{}{})
	if err != nil {
		return nil, nil, fmt.Errorf("putting empty object: %w", err)
	}

	sv, err := tree.VersionForNetwork(template.NetworkVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("getting state tree version: %w", err)
	}

	state, err := tree.NewState(cst, sv)
	if err != nil {
		return nil, nil, fmt.Errorf("making new state tree: %w", err)
	}

	av, err := actors.VersionForNetwork(template.NetworkVersion)
	if err != nil {
		return nil, nil, fmt.Errorf("get actor version: %w", err)
	}

	if err := builtinactors.LoadBundles(ctx, bs, av); err != nil {
		return nil, nil, fmt.Errorf("loading actors for genesis block: %w", err)
	}
	// Create system actor

	sysact, err := SetupSystemActor(ctx, bs, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup system actor: %w", err)
	}
	if err := state.SetActor(ctx, system.Address, sysact); err != nil {
		return nil, nil, fmt.Errorf("set system actor: %w", err)
	}

	// Create init actor

	idStart, initact, keyIDs, err := SetupInitActor(ctx, bs, template.NetworkName, template.Accounts, template.VerifregRootKey, template.RemainderAccount, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup init actor: %w", err)
	}
	if err := state.SetActor(ctx, init_.Address, initact); err != nil {
		return nil, nil, fmt.Errorf("set init actor: %w", err)
	}

	// Setup reward
	// RewardActor's state is overwritten by SetupStorageMiners, but needs to exist for miner creation messages
	rewact, err := SetupRewardActor(ctx, bs, big.Zero(), av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup reward actor: %w", err)
	}

	err = state.SetActor(ctx, reward.Address, rewact)
	if err != nil {
		return nil, nil, fmt.Errorf("set reward actor: %w", err)
	}

	// Setup cron
	cronact, err := SetupCronActor(ctx, bs, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup cron actor: %w", err)
	}
	if err := state.SetActor(ctx, cron.Address, cronact); err != nil {
		return nil, nil, fmt.Errorf("set cron actor: %w", err)
	}

	// Create empty power actor
	spact, err := SetupStoragePowerActor(ctx, bs, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup storage power actor: %w", err)
	}
	if err := state.SetActor(ctx, power.Address, spact); err != nil {
		return nil, nil, fmt.Errorf("set storage power actor: %w", err)
	}

	// Create empty market actor
	marketact, err := SetupStorageMarketActor(ctx, bs, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup storage market actor: %w", err)
	}
	if err := state.SetActor(ctx, market.Address, marketact); err != nil {
		return nil, nil, fmt.Errorf("set storage market actor: %w", err)
	}

	// Create verified registry
	verifact, err := SetupVerifiedRegistryActor(ctx, bs, av)
	if err != nil {
		return nil, nil, fmt.Errorf("setup verified registry market actor: %w", err)
	}
	if err := state.SetActor(ctx, verifreg.Address, verifact); err != nil {
		return nil, nil, fmt.Errorf("set verified registry actor: %w", err)
	}

	bact, err := makeAccountActor(ctx, cst, av, builtin.BurntFundsActorAddr, big.Zero())
	if err != nil {
		return nil, nil, fmt.Errorf("setup burnt funds actor state: %w", err)
	}
	if err := state.SetActor(ctx, builtin.BurntFundsActorAddr, bact); err != nil {
		return nil, nil, fmt.Errorf("set burnt funds actor: %w", err)
	}

	// Create accounts
	for _, info := range template.Accounts {

		switch info.Type {
		case TAccount:
			if err := createAccountActor(ctx, cst, state, info, keyIDs, av); err != nil {
				return nil, nil, fmt.Errorf("failed to create account actor: %w", err)
			}

		case TMultisig:

			ida, err := address.NewIDAddress(uint64(idStart))
			if err != nil {
				return nil, nil, err
			}
			idStart++

			if err := createMultisigAccount(ctx, cst, state, ida, info, keyIDs, av); err != nil {
				return nil, nil, err
			}
		default:
			return nil, nil, errors.New("unsupported account type")
		}

	}

	switch template.VerifregRootKey.Type {
	case TAccount:
		var ainfo AccountMeta
		if err := json.Unmarshal(template.VerifregRootKey.Meta, &ainfo); err != nil {
			return nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
		}

		_, ok := keyIDs[ainfo.Owner]
		if ok {
			return nil, nil, fmt.Errorf("rootkey account has already been declared, cannot be assigned 80: %s", ainfo.Owner)
		}

		vact, err := makeAccountActor(ctx, cst, av, ainfo.Owner, template.VerifregRootKey.Balance)
		if err != nil {
			return nil, nil, fmt.Errorf("setup verifreg rootkey account state: %w", err)
		}
		if err = state.SetActor(ctx, builtin.RootVerifierAddress, vact); err != nil {
			return nil, nil, fmt.Errorf("set verifreg rootkey account actor: %w", err)
		}
	case TMultisig:
		if err = createMultisigAccount(ctx, cst, state, builtin.RootVerifierAddress, template.VerifregRootKey, keyIDs, av); err != nil {
			return nil, nil, fmt.Errorf("failed to set up verified registry signer: %w", err)
		}
	default:
		return nil, nil, fmt.Errorf("unknown account type for verifreg rootkey: %w", err)
	}

	// Setup the first verifier as ID-address 81
	// TODO: remove this
	skBytes, err := sigs.Generate(crypto.SigTypeBLS)
	if err != nil {
		return nil, nil, fmt.Errorf("creating random verifier secret key: %w", err)
	}

	verifierPk, err := sigs.ToPublic(crypto.SigTypeBLS, skBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("creating random verifier public key: %w", err)
	}

	verifierAd, err := address.NewBLSAddress(verifierPk)
	if err != nil {
		return nil, nil, fmt.Errorf("creating random verifier address: %w", err)
	}

	verifierId, err := address.NewIDAddress(81) // nolint
	if err != nil {
		return nil, nil, err
	}

	verifierAct, err := makeAccountActor(ctx, cst, av, verifierAd, big.Zero())
	if err != nil {
		return nil, nil, fmt.Errorf("setup first verifier state: %w", err)
	}

	if err = state.SetActor(ctx, verifierId, verifierAct); err != nil {
		return nil, nil, fmt.Errorf("set first verifier actor: %w", err)
	}

	totalFilAllocated := big.Zero()

	err = state.ForEach(func(addr address.Address, act *types.Actor) error {
		if act.Balance.Nil() {
			panic(fmt.Sprintf("actor %s (%s) has nil balance", addr, builtin.ActorNameByCode(act.Code)))
		}
		totalFilAllocated = big.Add(totalFilAllocated, act.Balance)
		return nil
	})
	if err != nil {
		return nil, nil, fmt.Errorf("summing account balances in state tree: %w", err)
	}

	totalFil := big.Mul(big.NewInt(int64(constants.FilBase)), big.NewInt(int64(constants.FilecoinPrecision)))
	remainingFil := big.Sub(totalFil, totalFilAllocated)
	if remainingFil.Sign() < 0 {
		return nil, nil, fmt.Errorf("somehow overallocated filecoin (allocated = %s)", types.FIL(totalFilAllocated))
	}

	template.RemainderAccount.Balance = remainingFil

	switch template.RemainderAccount.Type {
	case TAccount:
		var ainfo AccountMeta
		if err := json.Unmarshal(template.RemainderAccount.Meta, &ainfo); err != nil {
			return nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
		}

		_, ok := keyIDs[ainfo.Owner]
		if ok {
			return nil, nil, fmt.Errorf("remainder account has already been declared, cannot be assigned 90: %s", ainfo.Owner)
		}

		keyIDs[ainfo.Owner] = builtin.ReserveAddress
		err = createAccountActor(ctx, cst, state, template.RemainderAccount, keyIDs, av)
		if err != nil {
			return nil, nil, fmt.Errorf("creating remainder acct: %w", err)
		}

	case TMultisig:
		if err = createMultisigAccount(ctx, cst, state, builtin.ReserveAddress, template.RemainderAccount, keyIDs, av); err != nil {
			return nil, nil, fmt.Errorf("failed to set up remainder: %w", err)
		}
	default:
		return nil, nil, fmt.Errorf("unknown account type for remainder: %w", err)
	}

	return state, keyIDs, nil
}

func makeAccountActor(ctx context.Context, cst cbor.IpldStore, av actors.Version, addr address.Address, bal types.BigInt) (*types.Actor, error) {
	ast, err := account.MakeState(adt.WrapStore(ctx, cst), av, addr)
	if err != nil {
		return nil, err
	}

	statecid, err := cst.Put(ctx, ast.GetState())
	if err != nil {
		return nil, err
	}

	actcid, found := actors.GetActorCodeID(av, actors.AccountKey)
	if !found {
		return nil, fmt.Errorf("failed to get account actor code ID for actors version %d", av)
	}

	act := &types.Actor{
		Code:    actcid,
		Head:    statecid,
		Balance: bal,
	}

	return act, nil
}

func createAccountActor(ctx context.Context, cst cbor.IpldStore, state *tree.State, info Actor, keyIDs map[address.Address]address.Address, av actors.Version) error {
	var ainfo AccountMeta
	if err := json.Unmarshal(info.Meta, &ainfo); err != nil {
		return fmt.Errorf("unmarshaling account meta: %w", err)
	}

	aa, err := makeAccountActor(ctx, cst, av, ainfo.Owner, info.Balance)
	if err != nil {
		return err
	}

	ida, ok := keyIDs[ainfo.Owner]
	if !ok {
		return fmt.Errorf("no registered ID for account actor: %s", ainfo.Owner)
	}

	err = state.SetActor(ctx, ida, aa)
	if err != nil {
		return fmt.Errorf("setting account from actmap: %w", err)
	}
	return nil
}

func createMultisigAccount(ctx context.Context, cst cbor.IpldStore, state *tree.State, ida address.Address, info Actor, keyIDs map[address.Address]address.Address, av actors.Version) error {
	if info.Type != TMultisig {
		return fmt.Errorf("can only call createMultisigAccount with multisig Actor info")
	}
	var ainfo MultisigMeta
	if err := json.Unmarshal(info.Meta, &ainfo); err != nil {
		return fmt.Errorf("unmarshaling account meta: %w", err)
	}

	var signers []address.Address

	for _, e := range ainfo.Signers {
		idAddress, ok := keyIDs[e]
		if !ok {
			return fmt.Errorf("no registered key ID for signer: %s", e)
		}

		// Check if actor already exists
		_, bFind, err := state.GetActor(ctx, e)
		if err == nil && bFind {
			signers = append(signers, idAddress)
			continue
		}

		aa, err := makeAccountActor(ctx, cst, av, e, big.Zero())
		if err != nil {
			return err
		}

		if err = state.SetActor(ctx, idAddress, aa); err != nil {
			return fmt.Errorf("setting account from actmap: %w", err)
		}
		signers = append(signers, idAddress)
	}

	mst, err := multisig.MakeState(adt.WrapStore(ctx, cst), av, signers, uint64(ainfo.Threshold), abi.ChainEpoch(ainfo.VestingStart), abi.ChainEpoch(ainfo.VestingDuration), info.Balance)
	if err != nil {
		return err
	}

	statecid, err := cst.Put(ctx, mst.GetState())
	if err != nil {
		return err
	}

	actcid, found := actors.GetActorCodeID(av, actors.MultisigKey)
	if !found {
		return fmt.Errorf("failed to get multisig actor code ID for actors version %d", av)
	}

	err = state.SetActor(ctx, ida, &types.Actor{
		Code:    actcid,
		Balance: info.Balance,
		Head:    statecid,
	})
	if err != nil {
		return fmt.Errorf("setting account from actmap: %w", err)
	}

	return nil
}

func VerifyPreSealedData(ctx context.Context, cs *chain.Store, stateroot cid.Cid, template Template, keyIDs map[address.Address]address.Address, nv network.Version, para *config.ForkUpgradeConfig) (cid.Cid, error) {
	verifNeeds := make(map[address.Address]abi.PaddedPieceSize)
	var sum abi.PaddedPieceSize

	faultChecker := consensusfault.NewFaultChecker(cs, fork.NewMockFork())
	syscalls := vmsupport.NewSyscalls(faultChecker, impl.ProofVerifier)

	csc := func(context.Context, abi.ChainEpoch, tree.Tree) (abi.TokenAmount, error) {
		return big.Zero(), nil
	}

	gasPriceSchedule := gas.NewPricesSchedule(para)
	vmopt := vm.VmOption{
		CircSupplyCalculator: csc,
		NetworkVersion:       nv,
		Rnd:                  &fakeRand{},
		BaseFee:              big.NewInt(0),
		Epoch:                0,
		PRoot:                stateroot,
		Bsstore:              cs.Blockstore(),
		SysCallsImpl:         mkFakedSigSyscalls(syscalls),
		GasPriceSchedule:     gasPriceSchedule,
	}

	vm, err := fvm.NewVM(ctx, vmopt)
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to create NewVenusVM: %w", err)
	}

	for mi, m := range template.Miners {
		for si, s := range m.Sectors {
			if s.Deal.Provider != m.ID {
				return cid.Undef, fmt.Errorf("sector %d in miner %d in template had mismatch in provider and miner ID: %s != %s", si, mi, s.Deal.Provider, m.ID)
			}

			amt := s.Deal.PieceSize
			verifNeeds[keyIDs[s.Deal.Client]] += amt
			sum += amt
		}
	}

	verifregRoot, err := address.NewIDAddress(80)
	if err != nil {
		return cid.Undef, err
	}

	verifier, err := address.NewIDAddress(81)
	if err != nil {
		return cid.Undef, err
	}

	// Note: This is brittle, if the methodNum / param changes, it could break things
	_, err = doExecValue(ctx, vm, verifreg.Address, verifregRoot, types.NewInt(0), builtin0.MethodsVerifiedRegistry.AddVerifier, mustEnc(&verifreg0.AddVerifierParams{

		Address:   verifier,
		Allowance: abi.NewStoragePower(int64(sum)), // eh, close enough

	}))
	if err != nil {
		return cid.Undef, fmt.Errorf("failed to create verifier: %w", err)
	}

	for c, amt := range verifNeeds {
		// Note: This is brittle, if the methodNum / param changes, it could break things
		_, err := doExecValue(ctx, vm, verifreg.Address, verifier, types.NewInt(0), builtin0.MethodsVerifiedRegistry.AddVerifiedClient, mustEnc(&verifreg0.AddVerifiedClientParams{
			Address:   c,
			Allowance: abi.NewStoragePower(int64(amt)),
		}))
		if err != nil {
			return cid.Undef, fmt.Errorf("failed to add verified client: %w", err)
		}
	}

	st, err := vm.Flush(ctx)
	if err != nil {
		return cid.Cid{}, fmt.Errorf("vm flush: %w", err)
	}

	return st, nil
}

func MakeGenesisBlock(ctx context.Context, rep repo.Repo, bs bstore.Blockstore, template Template, para *config.ForkUpgradeConfig) (*GenesisBootstrap, error) {
	st, keyIDs, err := MakeInitialStateTree(ctx, bs, template)
	if err != nil {
		return nil, fmt.Errorf("make initial state tree failed: %w", err)
	}

	stateroot, err := st.Flush(ctx)
	if err != nil {
		return nil, fmt.Errorf("flush state tree failed: %w", err)
	}

	// temp chainstore
	cs := chain.NewStore(rep.ChainDatastore(), bs, cid.Undef, chain.NewMockCirculatingSupplyCalculator())

	// Verify PreSealed Data
	stateroot, err = VerifyPreSealedData(ctx, cs, stateroot, template, keyIDs, template.NetworkVersion, para)
	if err != nil {
		return nil, fmt.Errorf("failed to verify presealed data: %w", err)
	}

	stateroot, err = SetupStorageMiners(ctx, cs, stateroot, template.Miners, template.NetworkVersion, para)
	if err != nil {
		return nil, fmt.Errorf("setup miners failed: %w", err)
	}

	store := adt.WrapStore(ctx, cbor.NewCborStore(bs))
	emptyroot, err := adt0.MakeEmptyArray(store).Root()
	if err != nil {
		return nil, fmt.Errorf("amt build failed: %w", err)
	}

	mm := &types.MessageRoot{
		BlsRoot:   emptyroot,
		SecpkRoot: emptyroot,
	}
	mmb, err := mm.ToStorageBlock()
	if err != nil {
		return nil, fmt.Errorf("serializing msgmeta failed: %w", err)
	}
	if err := bs.Put(ctx, mmb); err != nil {
		return nil, fmt.Errorf("putting msgmeta block to blockstore: %w", err)
	}

	log.Infof("Empty Genesis root: %s", emptyroot)

	tickBuf := make([]byte, 32)
	_, _ = rand.Read(tickBuf)
	genesisticket := &types.Ticket{
		VRFProof: tickBuf,
	}

	filecoinGenesisCid, err := cid.Decode("bafyreiaqpwbbyjo4a42saasj36kkrpv4tsherf2e7bvezkert2a7dhonoi")
	if err != nil {
		return nil, fmt.Errorf("failed to decode filecoin genesis block CID: %w", err)
	}

	if !expectedCid().Equals(filecoinGenesisCid) {
		return nil, fmt.Errorf("expectedCid != filecoinGenesisCid")
	}

	gblk, err := getGenesisBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to construct filecoin genesis block: %w", err)
	}

	if !filecoinGenesisCid.Equals(gblk.Cid()) {
		return nil, fmt.Errorf("filecoinGenesisCid != gblk.Cid")
	}

	if err := bs.Put(ctx, gblk); err != nil {
		return nil, fmt.Errorf("failed writing filecoin genesis block to blockstore: %w", err)
	}

	b := &types.BlockHeader{
		Miner:                 system.Address,
		Ticket:                genesisticket,
		Parents:               types.NewTipSetKey(filecoinGenesisCid).Cids(),
		Height:                0,
		ParentWeight:          types.NewInt(0),
		ParentStateRoot:       stateroot,
		Messages:              mmb.Cid(),
		ParentMessageReceipts: emptyroot,
		BLSAggregate:          nil,
		BlockSig:              nil,
		Timestamp:             template.Timestamp,
		ElectionProof:         new(types.ElectionProof),
		BeaconEntries: []types.BeaconEntry{
			{
				Round: 0,
				Data:  make([]byte, 32),
			},
		},
		ParentBaseFee: abi.NewTokenAmount(constants.InitialBaseFee),
	}

	sb, err := b.ToStorageBlock()
	if err != nil {
		return nil, fmt.Errorf("serializing block header failed: %w", err)
	}

	if err := bs.Put(ctx, sb); err != nil {
		return nil, fmt.Errorf("putting header to blockstore: %w", err)
	}

	return &GenesisBootstrap{
		Genesis: b,
	}, nil
}
