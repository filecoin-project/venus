package cmd

import (
	"testing"

	addr "github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/go-state-types/builtin/v8/paych"
	"github.com/filecoin-project/specs-actors/actors/builtin"
	"github.com/filecoin-project/specs-actors/actors/runtime"
	tutil "github.com/filecoin-project/specs-actors/support/testing"
	"github.com/stretchr/testify/assert"
)

func TestEncodedString(t *testing.T) {
	mnum := builtin.MethodsPaych.UpdateChannelState
	fakeParams := runtime.CBORBytes([]byte{1, 2, 3, 4})
	otherAddr := tutil.NewIDAddr(t, 104)
	ex := &paych.ModVerifyParams{
		Actor:  otherAddr,
		Method: mnum,
		Data:   fakeParams,
	}
	chanAddr, _ := addr.NewFromString("t15ihq5ibzwki2b4ep2f46avlkrqzhpqgtga7pdrq")
	sv := &paych.SignedVoucher{
		ChannelAddr:     chanAddr,
		TimeLockMin:     1,
		TimeLockMax:     100,
		SecretHash:      []byte("ProfesrXXXXXXXXXXXXXXXXXXXXXXXXX"),
		Extra:           ex,
		Lane:            1,
		Nonce:           1,
		Amount:          big.NewInt(10),
		MinSettleHeight: 1000,
		Merges:          nil,
	}
	str, err := encodedString(sv)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, str, "i1UB6g8OoDmykaDwj9F54FVqjDJ3wNMBGGRYIFByb2Zlc3JYWFhYWFhYWFhYWFhYWFhYWFhYWFhYWFhYg0IAaAJEAQIDBAEBQgAKGQPogPY")
}
