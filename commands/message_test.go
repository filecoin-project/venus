package commands

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
)

func TestMessageSend(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Log("[failure] invalid target")
	d.RunFail(
		"invalid checksum",
		"message", "send",
		"--from", core.NetworkAccount.String(),
		"--value=10", "xyz",
	)

	t.Log("[failure] no from and no addresses")
	d.RunFail(
		"no default address",
		"message", "send", core.TestAccount.String(),
	)

	t.Log("[success] no from")
	d.RunSuccess("wallet addrs new")
	d.RunSuccess("message", "send", core.TestAccount.String())

	t.Log("[success] with from")
	d.RunSuccess("message", "send",
		"--from", core.NetworkAccount.String(), core.TestAccount.String(),
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message", "send",
		"--from", core.NetworkAccount.String(),
		"--value=10", core.TestAccount.String(),
	)
}
