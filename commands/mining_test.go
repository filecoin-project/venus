package commands

import (
	"testing"
)

func TestMinerGenBlock(t *testing.T) {
	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	t.Log("[failure] no addresses")
	d.RunFail("no addresses in wallet to mine", "mining once")

	t.Log("[success] address in local wallet")
	d.RunSuccess("wallet addrs new")
	d.RunSuccess("mining once")
}
