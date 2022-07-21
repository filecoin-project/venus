package genesis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"

	"github.com/filecoin-project/specs-actors/actors/util/adt"

	cbor "github.com/ipfs/go-ipld-cbor"
	cbg "github.com/whyrusleeping/cbor-gen"

	bstore "github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	"github.com/filecoin-project/venus/venus-shared/actors"
	init_ "github.com/filecoin-project/venus/venus-shared/actors/builtin/init"
	"github.com/filecoin-project/venus/venus-shared/types"
)

func SetupInitActor(ctx context.Context, bs bstore.Blockstore, netname string, initialActors []Actor, rootVerifier Actor, remainder Actor, av actors.Version) (int64, *types.Actor, map[address.Address]address.Address, error) {
	if len(initialActors) > MaxAccounts {
		return 0, nil, nil, errors.New("too many initial actors")
	}

	cst := cbor.NewCborStore(bs)
	ist, err := init_.MakeState(adt.WrapStore(ctx, cst), av, netname)
	if err != nil {
		return 0, nil, nil, err
	}

	if err = ist.SetNextID(MinerStart); err != nil {
		return 0, nil, nil, err
	}

	amap, err := ist.AddressMap()
	if err != nil {
		return 0, nil, nil, err
	}

	keyToID := map[address.Address]address.Address{}
	counter := int64(AccountStart)

	for _, a := range initialActors {
		if a.Type == TMultisig {
			var ainfo MultisigMeta
			if err := json.Unmarshal(a.Meta, &ainfo); err != nil {
				return 0, nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
			}
			for _, e := range ainfo.Signers {

				if _, ok := keyToID[e]; ok {
					continue
				}

				fmt.Printf("init set %s t0%d\n", e, counter)

				value := cbg.CborInt(counter)
				if err := amap.Put(abi.AddrKey(e), &value); err != nil {
					return 0, nil, nil, err
				}
				counter = counter + 1
				var err error
				keyToID[e], err = address.NewIDAddress(uint64(value))
				if err != nil {
					return 0, nil, nil, err
				}

			}
			// Need to add actors for all multisigs too
			continue
		}

		if a.Type != TAccount {
			return 0, nil, nil, fmt.Errorf("unsupported account type: %s", a.Type)
		}

		var ainfo AccountMeta
		if err := json.Unmarshal(a.Meta, &ainfo); err != nil {
			return 0, nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
		}

		fmt.Printf("init set %s t0%d\n", ainfo.Owner, counter)

		value := cbg.CborInt(counter)
		if err := amap.Put(abi.AddrKey(ainfo.Owner), &value); err != nil {
			return 0, nil, nil, err
		}
		counter = counter + 1

		var err error
		keyToID[ainfo.Owner], err = address.NewIDAddress(uint64(value))
		if err != nil {
			return 0, nil, nil, err
		}
	}

	setupMsig := func(meta json.RawMessage) error {
		var ainfo MultisigMeta
		if err := json.Unmarshal(meta, &ainfo); err != nil {
			return fmt.Errorf("unmarshaling account meta: %w", err)
		}
		for _, e := range ainfo.Signers {
			if _, ok := keyToID[e]; ok {
				continue
			}
			fmt.Printf("init set %s t0%d\n", e, counter)

			value := cbg.CborInt(counter)
			if err := amap.Put(abi.AddrKey(e), &value); err != nil {
				return err
			}
			counter = counter + 1
			var err error
			keyToID[e], err = address.NewIDAddress(uint64(value))
			if err != nil {
				return err
			}

		}

		return nil
	}

	if rootVerifier.Type == TAccount {
		var ainfo AccountMeta
		if err := json.Unmarshal(rootVerifier.Meta, &ainfo); err != nil {
			return 0, nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
		}
		value := cbg.CborInt(80)
		if err := amap.Put(abi.AddrKey(ainfo.Owner), &value); err != nil {
			return 0, nil, nil, err
		}
	} else if rootVerifier.Type == TMultisig {
		err := setupMsig(rootVerifier.Meta)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("setting up root verifier msig: %w", err)
		}
	}

	if remainder.Type == TAccount {
		var ainfo AccountMeta
		if err := json.Unmarshal(remainder.Meta, &ainfo); err != nil {
			return 0, nil, nil, fmt.Errorf("unmarshaling account meta: %w", err)
		}

		// TODO: Use builtin.ReserveAddress...
		value := cbg.CborInt(90)
		if err := amap.Put(abi.AddrKey(ainfo.Owner), &value); err != nil {
			return 0, nil, nil, err
		}
	} else if remainder.Type == TMultisig {
		err := setupMsig(remainder.Meta)
		if err != nil {
			return 0, nil, nil, fmt.Errorf("setting up remainder msig: %w", err)
		}
	}

	amapaddr, err := amap.Root()
	if err != nil {
		return 0, nil, nil, err
	}

	if err = ist.SetAddressMap(amapaddr); err != nil {
		return 0, nil, nil, err
	}

	statecid, err := cst.Put(ctx, ist.GetState())
	if err != nil {
		return 0, nil, nil, err
	}

	actcid, found := actors.GetActorCodeID(av, actors.InitKey)
	if !found {
		return 0, nil, nil, fmt.Errorf("failed to get init actor code ID for actors version %d", av)
	}

	act := &types.Actor{
		Code:    actcid,
		Head:    statecid,
		Balance: big.Zero(),
	}

	return counter, act, keyToID, nil
}
