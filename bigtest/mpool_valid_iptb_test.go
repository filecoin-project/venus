package bigtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/types"
)

func TestMpoolSync(t *testing.T) {
	// must pass "-bigtest=true" flag to run
	if !*bigtest {
		t.SkipNow()
	}
	require := require.New(t)
	assert := assert.New(t)

	// things to fiddle with
	// would be cool if `msgPropDuration` was derived from `numNodes`
	numNodes := 5
	msgPropDuration := 5 * time.Second

	// initialize numNodes filecoin nodes locally and start them
	nodes := MustStartIPTB(require, numNodes, "filecoin", "local")
	defer MustKillIPTB(require, nodes)

	MustConnectIPTB(require, nodes)

	// every node submit a bid & ensure the others got the bid message
	// before proceeding
	for _, n := range nodes {
		// TODO test with more than just bids
		out := MustRunIPTB(require, n,
			"go-filecoin", "client",
			"add-bid", "1", "1", fmt.Sprintf("--from=%s", address.TestAddress))

		MustHaveCidInPoolBy(require, out, msgPropDuration, nodes)
	}

	// capture the state of every nodes message pool
	nodePools := make(map[int][]types.Message)
	for i, n := range nodes {
		out := MustRunIPTB(require, n, "go-filecoin", "mpool", "--enc=json")
		nodePools[i] = MustUnmarshalMessages(require, out)
	}
	expected := nodePools[0]
	MustHaveEqualMessages(assert, expected, nodePools)
}
