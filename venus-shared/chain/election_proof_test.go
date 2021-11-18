package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/venus-shared/chain/params"
	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestElectionProofBasic(t *testing.T) {
	vrfLen := 32
	winCountMin := int64(3)
	winCountMax := int64(10)
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst ElectionProof

		var power abi.StoragePower
		testutil.Provide(t, &power, testutil.PositiveBigProvider())
		require.True(t, power.GreaterThan(big.Zero()), "positive storage power")
		totalPower := BigMul(power, NewInt(7))

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(vrfLen),
				testutil.IntRangedProvider(int(winCountMin), int(winCountMax)),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "src value provided")
				require.Len(t, src.VRFProof, vrfLen, "vrf length")
				require.GreaterOrEqual(t, src.WinCount, winCountMin, "win count min")
				require.Less(t, src.WinCount, winCountMax, "win count max")
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")
				winCount := src.ComputeWinCount(power, totalPower)
				require.GreaterOrEqual(t, winCount, int64(0), "win count >=0")
				require.LessOrEqual(t, winCount, params.MaxWinCount, "win count <= MaxWinCount")
				require.Equal(t, winCount, dst.ComputeWinCount(power, totalPower))

				t1, t2 := Ticket{
					VRFProof: src.VRFProof,
				}, Ticket{
					VRFProof: dst.VRFProof,
				}

				require.Equal(t, t1, t2, "ticket")

				require.True(t, t1.Compare(&t2) == 0, "ticket equal")
				require.Equal(t, t1.Quality(), t2.Quality())

			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}

func TestTicketBasic(t *testing.T) {
	vrfLen := 32
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst, another Ticket

		opt := testutil.CborErBasicTestOptions{
			Buf: &buf,
			Prepare: func() {
				require.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(vrfLen),
			},

			Provided: func() {
				require.NotEqual(t, src, dst, "src value provided")
				require.Len(t, src.VRFProof, vrfLen, "vrf length")
			},

			Finished: func() {
				require.Equal(t, src, dst, "from src to dst through cbor")

				t1, t2 := Ticket{
					VRFProof: src.VRFProof,
				}, Ticket{
					VRFProof: dst.VRFProof,
				}

				require.Equal(t, t1, t2, "ticket")

				require.True(t, t1.Compare(&t2) == 0, "ticket equal")
				require.Equal(t, t1.Quality(), t2.Quality())
				require.Equal(t, t1.String(), t2.String(), "ticket string")

				testutil.Provide(t, &another, testutil.BytesFixedProvider(vrfLen))
				require.Len(t, another.VRFProof, vrfLen, "vrf length")
				require.True(t, src.Less(&another) == (src.Compare(&another) < 0))
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
