package chain

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

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
				assert.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(vrfLen),
				testutil.IntRangedProvider(int(winCountMin), int(winCountMax)),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "src value provided")
				assert.Len(t, src.VRFProof, vrfLen, "vrf length")
				assert.GreaterOrEqual(t, src.WinCount, winCountMin, "win count min")
				assert.Less(t, src.WinCount, winCountMax, "win count max")
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")

				t1, t2 := Ticket{
					VRFProof: src.VRFProof,
				}, Ticket{
					VRFProof: dst.VRFProof,
				}

				assert.Equal(t, t1, t2, "ticket")

				assert.True(t, t1.Compare(&t2) == 0, "ticket equal")
				assert.Equal(t, t1.Quality(), t2.Quality())

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
				assert.Equal(t, src, dst, "empty values")
			},

			ProvideOpts: []interface{}{
				testutil.BytesFixedProvider(vrfLen),
			},

			Provided: func() {
				assert.NotEqual(t, src, dst, "src value provided")
				assert.Len(t, src.VRFProof, vrfLen, "vrf length")
			},

			Finished: func() {
				assert.Equal(t, src, dst, "from src to dst through cbor")

				t1, t2 := Ticket{
					VRFProof: src.VRFProof,
				}, Ticket{
					VRFProof: dst.VRFProof,
				}

				assert.Equal(t, t1, t2, "ticket")

				assert.True(t, t1.Compare(&t2) == 0, "ticket equal")
				assert.Equal(t, t1.Quality(), t2.Quality())

				testutil.Provide(t, &another, testutil.BytesFixedProvider(vrfLen))
				assert.Len(t, another.VRFProof, vrfLen, "vrf length")
				assert.True(t, src.Less(&another) == (src.Compare(&another) < 0))
			},
		}

		testutil.CborErBasicTest(t, &src, &dst, opt)
	}
}
