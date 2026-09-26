package genesis_test

import (
	"context"
	"testing"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	actorstypes "github.com/filecoin-project/go-state-types/actors"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/fixtures/networks"
	"github.com/filecoin-project/venus/pkg/gen"
	"github.com/filecoin-project/venus/pkg/gen/genesis"
	bstore "github.com/filecoin-project/venus/venus-shared/blockstore"
)

// Genesis creation must work at the network's own genesis version (devnet genesis is
// Version28 since nv29), which needs both a state tree version and an actors version
// resolvable for that network version.
func TestMakeInitialStateTreeAtDevnetGenesisVersion(t *testing.T) {
	signer, err := address.NewFromString("t1ceb34gnsc6qk5dt6n7xg6ycwzasjhbxm3iylkiy")
	require.NoError(t, err)

	for _, c := range []struct {
		name string
		nc   *networks.NetworkConf
	}{
		{"2k", networks.Net2k()},
		{"butterfly", networks.ButterflySnapNet()},
	} {
		t.Run(c.name, func(t *testing.T) {
			ctx := context.Background()
			bs := bstore.NewTemporary()

			template := genesis.Template{
				NetworkVersion:   c.nc.Network.GenesisNetworkVersion,
				NetworkName:      "probe",
				Accounts:         []genesis.Actor{{Type: genesis.TAccount, Balance: abi.NewTokenAmount(0), Meta: (&genesis.AccountMeta{Owner: signer}).ActorMeta()}},
				VerifregRootKey:  gen.DefaultVerifregRootkeyActor,
				RemainderAccount: gen.DefaultRemainderAccountActor,
			}

			st, keyIDs, err := genesis.MakeInitialStateTree(ctx, bs, template)
			require.NoErrorf(t, err, "network=%s genesisNetworkVersion=%d", c.name, template.NetworkVersion)

			root, err := st.Flush(ctx)
			require.NoError(t, err)

			av, err := actorstypes.VersionForNetwork(template.NetworkVersion)
			require.NoError(t, err)

			t.Logf("network=%s genesisNetworkVersion=%d actorsVersion=%d accounts=%d stateRoot=%s",
				c.name, template.NetworkVersion, int(av), len(keyIDs), root)
		})
	}
}
