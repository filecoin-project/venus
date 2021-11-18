package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/venus/venus-shared/testutil"
)

func TestElectionProofBasic(t *testing.T) {
	vrfLen := 32
	winCountMin := int64(3)
	winCountMax := int64(10)
	var buf bytes.Buffer
	for i := 0; i < 32; i++ {
		var src, dst ElectionProof

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

				testutil.Provide(t, &another, testutil.BytesFixedProvider(vrfLen))
				require.Len(t, another.VRFProof, vrfLen, "vrf length")
				require.True(t, src.Less(&another) == (src.Compare(&another) < 0))
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
