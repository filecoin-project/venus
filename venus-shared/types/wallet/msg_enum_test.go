package wallet

import (
	"testing"

	tf "github.com/filecoin-project/venus/pkg/testhelpers/testflags"
	"github.com/filecoin-project/venus/venus-shared/types"
	"gotest.tools/assert"
)

func TestContainMsgType(t *testing.T) {
	tf.UnitTest(t)
	multiME := MEUnknown + MEChainMsg + MEStorageAsk + MEProviderDealState + MEVerifyAddress
	assert.Equal(t, ContainMsgType(multiME, types.MTChainMsg), true)
	assert.Equal(t, ContainMsgType(multiME, types.MTStorageAsk), true)
	assert.Equal(t, ContainMsgType(multiME, types.MTProviderDealState), true)
	assert.Equal(t, ContainMsgType(multiME, types.MTUnknown), true)
	assert.Equal(t, ContainMsgType(multiME, types.MTBlock), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTDealProposal), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTDrawRandomParam), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTSignedVoucher), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTAskResponse), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTNetWorkResponse), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTClientDeal), false)
	assert.Equal(t, ContainMsgType(multiME, types.MTVerifyAddress), true)
}

func TestFindCode(t *testing.T) {
	tf.UnitTest(t)
	ids := FindCode(38)
	assert.DeepEqual(t, []int{1, 2, 5}, ids)

	ids2 := FindCode(8392)
	assert.DeepEqual(t, []int{3, 6, 7, 13}, ids2)
}

func TestAggregateMsgEnumCode(t *testing.T) {
	tf.UnitTest(t)
	me, err := AggregateMsgEnumCode([]int{1, 2, 3, 4, 5, 6, 7})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, me, uint32(254))
}
