package cmd

import (
	"bytes"
	"compress/flate"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"time"

	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/node"
	"github.com/filecoin-project/venus/pkg/vf3"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

var f3Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"check-activation-raw": f3CheckActivationRaw,
		"check-activation":     f3CheckActivation,
	},
}

var f3CheckActivation = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "queries f3 parameters contract using chain module",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("contract", true, false, "address contract to query"),
	},
	Options: []cmds.Option{
		cmds.StringOption("network-name", "network name").WithDefault("filecoin"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		contract := req.Arguments[0]
		ctx := requestContext(req)

		networkName, _ := req.Options["network-name"].(string)
		config := vf3.Config{
			BaseNetworkName:      gpbft.NetworkName(networkName),
			ContractAddress:      contract,
			ContractPollInterval: 15 * time.Second,
		}

		prov, err := vf3.NewManifestProvider(ctx, &config, nil, nil, nil, env.(*node.Env).ChainAPI)
		if err != nil {
			return fmt.Errorf("creating manifest proivder: %w", err)
		}

		err = prov.Start(ctx)
		if err != nil {
			return fmt.Errorf("starting manifest provider: %w", err)
		}
		for {
			select {
			case m := <-prov.ManifestUpdates():
				_ = re.Emit(m)
			case <-ctx.Done():
				_ = prov.Stop(context.Background())
				return nil
			}
		}
	},
}

var f3CheckActivationRaw = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "queries f3 parameters contract using raw logic",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("contract", true, false, "address contract to query"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		contract := req.Arguments[0]
		ctx := requestContext(req)

		// this code is raw logic for cross-checking
		// the cleaner code is in chain/lf3/manifest.go
		address, err := types.ParseEthAddress(contract)
		if err != nil {
			return fmt.Errorf("trying to parse contract address: %s: %w", contract, err)
		}

		ethCall := types.EthCall{
			To:   &address,
			Data: utils.One(types.DecodeHexString("0x2587660d")), // method ID of activationInformation()
		}
		fMessage, err := ethCall.ToFilecoinMessage()
		if err != nil {
			return fmt.Errorf("converting to filecoin message: %w", err)
		}

		msgRes, err := env.(*node.Env).ChainAPI.StateCall(ctx, fMessage, types.EmptyTSK)
		if err != nil {
			return fmt.Errorf("state call error: %w", err)
		}
		if msgRes.MsgRct.ExitCode != 0 {
			return fmt.Errorf("message returned exit code: %v", msgRes.MsgRct.ExitCode)
		}

		var ethReturn abi.CborBytes
		err = ethReturn.UnmarshalCBOR(bytes.NewReader(msgRes.MsgRct.Return))
		if err != nil {
			return fmt.Errorf("could not decode return value: %w", err)
		}
		_ = printOneString(re, fmt.Sprintf("Raw data: %X", ethReturn))
		slot, retBytes := []byte{}, []byte(ethReturn)
		_ = slot
		// 3*32 because there should be 3 slots minimum
		if len(retBytes) < 3*32 {
			return fmt.Errorf("no activation information")
		}

		// split off first slot
		slot, retBytes = retBytes[:32], retBytes[32:]
		// it is uint64 so we want the last 8 bytes
		slot = slot[24:32]
		activationEpoch := binary.BigEndian.Uint64(slot)
		_ = activationEpoch

		slot, retBytes = retBytes[:32], retBytes[32:]
		for i := range 31 {
			if slot[i] != 0 {
				return fmt.Errorf("wrong value for offest (padding): slot[%d] = 0x%x != 0x00", i, slot[i])
			}
		}
		if slot[31] != 0x40 {
			return fmt.Errorf("wrong value for offest : slot[31] = 0x%x != 0x40", slot[31])
		}
		slot, retBytes = retBytes[:32], retBytes[32:]
		slot = slot[24:32]
		pLen := binary.BigEndian.Uint64(slot)
		if pLen > 4<<10 {
			return fmt.Errorf("too long declared payload: %d > %d", pLen, 4<<10)
		}
		payloadLength := int(pLen)

		if payloadLength > len(retBytes) {
			return fmt.Errorf("not enough remaining bytes: %d > %d", payloadLength, retBytes)
		}

		if activationEpoch == math.MaxUint64 || payloadLength == 0 {
			return printOneString(re, "no active activation")
		} else {
			compressedManifest := retBytes[:payloadLength]
			reader := io.LimitReader(flate.NewReader(bytes.NewReader(compressedManifest)), 1<<20)
			var m manifest.Manifest
			err = json.NewDecoder(reader).Decode(&m)
			if err != nil {
				return fmt.Errorf("got error while decoding manifest: %w", err)
			}

			if m.BootstrapEpoch < 0 || uint64(m.BootstrapEpoch) != activationEpoch {
				return fmt.Errorf("bootstrap epoch does not match: %d != %d", m.BootstrapEpoch, activationEpoch)
			}
			buf := new(bytes.Buffer)
			_, _ = io.Copy(buf, flate.NewReader(bytes.NewReader(compressedManifest)))
			return re.Emit(buf)
		}
	},
}
