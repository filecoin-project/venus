package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/filecoin-project/go-state-types/big"
	"github.com/filecoin-project/venus/app/node"
	v1 "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var infoCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Print node info",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := req.Context
		chainapi := env.(*node.Env).ChainAPI
		buf := new(bytes.Buffer)
		writer := NewSilentWriter(buf)

		netParams, err := chainapi.StateGetNetworkParams(ctx)
		if err != nil {
			return err
		}
		writer.Printf("Network: %s\n", netParams.NetworkName)

		if err := SyncBasefeeCheck(ctx, chainapi, int64(netParams.BlockDelaySecs), writer); err != nil {
			return err
		}
		status, err := env.(*node.Env).CommonAPI.NodeStatus(ctx, true)
		if err != nil {
			return err
		}
		writer.Printf("Peers to: [publish messages %d] [publish blocks %d]\n", status.PeerStatus.PeersToPublishMsgs,
			status.PeerStatus.PeersToPublishBlocks)

		//Chain health calculated as percentage: amount of blocks in last finality / very healthy amount of blocks in a finality (900 epochs * 5 blocks per tipset)
		health := (100 * (900 * status.ChainStatus.BlocksPerTipsetLastFinality) / (900 * 5))
		switch {
		case health > 85:
			writer.Printf("Chain health: %.f%% [healthy]\n", health)
		case health < 85:
			writer.Printf("Chain health: %.f%% [unhealthy]\n", health)
		}
		writer.Println()

		addr, err := env.(*node.Env).WalletAPI.WalletDefaultAddress(ctx)
		if err == nil && !addr.Empty() {
			fmt.Printf("Default address: \n")
			balance, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, addr)
			if err != nil {
				return err
			}
			writer.Printf("      %s [%s]\n", addr.String(), types.FIL(balance).Short())
		} else {
			writer.Printf("Default address: address not set\n")
		}
		writer.Println()

		addrs := env.(*node.Env).WalletAPI.WalletAddresses(ctx)
		totalBalance := big.Zero()
		for _, addr := range addrs {
			totbal, err := env.(*node.Env).WalletAPI.WalletBalance(ctx, addr)
			if err != nil {
				return err
			}
			totalBalance = big.Add(totalBalance, totbal)
		}
		writer.Printf("Wallet: %v address\n", len(addrs))
		writer.Printf("      Total balance: %s\n", types.FIL(totalBalance).Short())

		mbLockedSum := big.Zero()
		mbAvailableSum := big.Zero()
		for _, addr := range addrs {
			mbal, err := env.(*node.Env).ChainAPI.StateMarketBalance(ctx, addr, types.EmptyTSK)
			if err != nil {
				if strings.Contains(err.Error(), "actor not found") {
					continue
				}
				return err
			}
			mbLockedSum = big.Add(mbLockedSum, mbal.Locked)
			mbAvailableSum = big.Add(mbAvailableSum, mbal.Escrow)
		}
		writer.Printf("      Market locked: %s\n", types.FIL(mbLockedSum).Short())
		writer.Printf("      Market available: %s\n", types.FIL(mbAvailableSum).Short())
		writer.Println()

		chs, err := env.(*node.Env).PaychAPI.PaychList(ctx)
		if err != nil {
			return err
		}
		writer.Printf("Payment Channels: %v channels\n", len(chs))
		writer.Println()

		s, err := env.(*node.Env).NetworkAPI.NetBandwidthStats(ctx)
		if err != nil {
			return err
		}
		tw := tabwriter.NewWriter(writer.w, 6, 6, 2, ' ', 0)
		writer.Printf("Bandwidth:\n")
		fmt.Fprintf(tw, "\tTotalIn\tTotalOut\tRateIn\tRateOut\n")
		fmt.Fprintf(tw, "\t%s\t%s\t%s/s\t%s/s\n", humanize.Bytes(uint64(s.TotalIn)), humanize.Bytes(uint64(s.TotalOut)), humanize.Bytes(uint64(s.RateIn)), humanize.Bytes(uint64(s.RateOut)))
		if err := tw.Flush(); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

func SyncBasefeeCheck(ctx context.Context, chainapi v1.IChain, blockDelaySecs int64, writer *SilentWriter) error {
	head, err := chainapi.ChainHead(ctx)
	if err != nil {
		return err
	}

	var syncStatus string
	switch {
	case time.Now().Unix()-int64(head.MinTimestamp()) < blockDelaySecs*3/2: // within 1.5 epochs
		syncStatus = "[sync ok]"
	case time.Now().Unix()-int64(head.MinTimestamp()) < blockDelaySecs*5: // within 5 epochs
		syncStatus = fmt.Sprintf("[sync slow (%s behind)]", time.Since(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))
	default:
		syncStatus = fmt.Sprintf("[sync behind! (%s behind)]", time.Since(time.Unix(int64(head.MinTimestamp()), 0)).Truncate(time.Second))
	}
	basefee := head.MinTicketBlock().ParentBaseFee

	writer.Printf("Chain: %s [basefee %s] [epoch %v]\n", syncStatus, types.FIL(basefee).Short(), head.Height())

	return nil
}
