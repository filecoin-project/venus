package commands

import (
	"testing"

	"github.com/filecoin-project/go-filecoin/types"
)

func TestMessageSend(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Log("[failure] invalid target")
	d.RunFail(
		"addresses must start with 0x",
		"message send",
		"--from", types.Address("filecoin").String(),
		"--value 10 xyz",
	)

	t.Log("[failure] no from and no addresses")
	d.RunFail(
		"no default address",
		"message send", types.Address("investor1").String(),
	)

	t.Log("[success] no from")
	d.RunSuccess("wallet addrs new")
	d.RunSuccess("message send", types.Address("investor1").String())

	t.Log("[success] with from")
	d.RunSuccess("message send",
		"--from", types.Address("filecoin").String(), types.Address("investor1").String(),
	)

	t.Log("[success] with from and value")
	d.RunSuccess("message send",
		"--from", types.Address("filecoin").String(),
		"--value 10", types.Address("investor1").String(),
	)
}
