package commands

import (
	"fmt"
	"strings"
	"testing"

	"github.com/filecoin-project/go-filecoin/core"
	"github.com/stretchr/testify/assert"
)

func TestAddrsNewAndList(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()

	addrs := make([]string, 10)
	for i := 0; i < 10; i++ {
		addrs[i] = d.CreateWalletAddr()
	}

	list := d.RunSuccess("wallet", "addrs", "ls").ReadStdout()
	for _, addr := range addrs {
		assert.Contains(list, addr)
	}
}

func TestWalletBalance(t *testing.T) {
	assert := assert.New(t)

	d := NewDaemon(t).Start()
	defer d.ShutdownSuccess()
	addr := d.CreateWalletAddr()

	t.Log("[failure] not found")
	d.RunFail("not found", "wallet", "balance", addr)

	t.Log("[success] balance 100000")
	balance := d.RunSuccess("wallet", "balance", core.NetworkAccount.String())
	assert.Contains(balance.ReadStdout(), "100000")
}

func TestAddrsLookup(t *testing.T) {
	assert := assert.New(t)

	//Define 2 nodes, each with an address
	d1 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6000")).Start()
	defer d1.ShutdownSuccess()
	d1.CreateWalletAddr()

	d2 := NewDaemon(t, SwarmAddr("/ip4/127.0.0.1/tcp/6001")).Start()
	defer d2.ShutdownSuccess()
	d2.CreateWalletAddr()

	//Connect daemons
	d1.ConnectSuccess(d2)

	d1Raw := d1.RunSuccess("address ls")
	d1Addrs := strings.Split(strings.Trim(d1Raw.ReadStdout(), "\n"), "\n")
	d1WalletAddr := d1Addrs[len(d1Addrs)-1]
	t.Logf("D1 Wallet Address: %s", d1WalletAddr)
	assert.NotEmpty(d1WalletAddr)

	d2Raw := d2.RunSuccess("address ls")
	d2Addrs := strings.Split(strings.Trim(d2Raw.ReadStdout(), "\n"), "\n")
	d2WalletAddr := d2Addrs[len(d2Addrs)-1]
	t.Logf("D2 Wallet Address: %s", d2WalletAddr)
	assert.NotEmpty(d2WalletAddr)

	isD2IdRaw := d1.RunSuccess(fmt.Sprintf("address lookup %s", d2WalletAddr))
	isD1IdRaw := d2.RunSuccess(fmt.Sprintf("address lookup %s", d1WalletAddr))

	isD1Id := strings.Trim(isD1IdRaw.ReadStdout(), "\n")
	isD2Id := strings.Trim(isD2IdRaw.ReadStdout(), "\n")

	assert.Equal(d1.GetID(), isD1Id)
	assert.Equal(d2.GetID(), isD2Id)
}
