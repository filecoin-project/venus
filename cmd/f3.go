package cmd

import (
	"bytes"
	"compress/flate"
	"context"
	"embed"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"text/template"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-f3/certs"
	"github.com/filecoin-project/go-f3/gpbft"
	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/venus/app/node"
	v1api "github.com/filecoin-project/venus/venus-shared/api/chain/v1"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/utils"
	"github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
)

//go:embed templates/f3_*.go.tmpl
var f3TemplatesFS embed.FS
var f3Templates = template.Must(
	template.New("").
		Funcs(template.FuncMap{
			"ptDiffToString":            f3PowerTableDiffsToString,
			"tipSetKeyToLotusTipSetKey": types.TipSetKeyFromBytes,
			"add":                       func(a, b int) int { return a + b },
			"sub":                       func(a, b int) int { return a - b },
		}).
		ParseFS(f3TemplatesFS, "templates/f3_*.go.tmpl"),
)

func f3PowerTableDiffsToString(diff certs.PowerTableDiff) (string, error) {
	if len(diff) == 0 {
		return "None", nil
	}
	totalDiff := gpbft.NewStoragePower(0).Int
	for _, delta := range diff {
		if !delta.IsZero() {
			totalDiff = totalDiff.Add(totalDiff, delta.PowerDelta.Int)
		}
	}
	if totalDiff.Cmp(gpbft.NewStoragePower(0).Int) == 0 {
		return "None", nil
	}
	return fmt.Sprintf("Total of %s storage power across %d miner(s).", totalDiff, len(diff)), nil
}

var f3Cmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Interact with filecoin blockchain",
	},
	Subcommands: map[string]*cmds.Command{
		"check-activation-raw": f3CheckActivationRaw,
		"status":               f3Status,
		"power-table":          f3SubCmdPowerTable,
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
				return fmt.Errorf("wrong value for offset (padding): slot[%d] = 0x%x != 0x00", i, slot[i])
			}
		}
		if slot[31] != 0x40 {
			return fmt.Errorf("wrong value for offset : slot[31] = 0x%x != 0x40", slot[31])
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

var f3Status = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Checks the F3 status.",
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := requestContext(req)

		api := env.(*node.Env).F3API
		running, err := api.F3IsRunning(ctx)
		if err != nil {
			return fmt.Errorf("getting running state: %w", err)
		}
		buf := new(bytes.Buffer)
		_, _ = fmt.Fprintf(buf, "Running: %t\n", running)
		if !running {
			return nil
		}

		progress, err := api.F3GetProgress(ctx)
		if err != nil {
			return fmt.Errorf("getting progress: %w", err)
		}

		_, _ = fmt.Fprintln(buf, "Progress:")
		_, _ = fmt.Fprintf(buf, "  Instance: %d\n", progress.ID)
		_, _ = fmt.Fprintf(buf, "  Round:    %d\n", progress.Round)
		_, _ = fmt.Fprintf(buf, "  Phase:    %s\n", progress.Phase)

		manifest, err := api.F3GetManifest(ctx)
		if err != nil {
			return fmt.Errorf("getting manifest: %w", err)
		}

		if err := prettyPrintManifest(buf, manifest); err != nil {
			return err
		}

		return re.Emit(buf)
	},
}

var f3SubCmdPowerTable = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Manages interactions with F3 power tables.",
	},
	Subcommands: map[string]*cmds.Command{
		"get":            f3SubCmdPowerTableGet,
		"get-proportion": f3SubCmdPowerTableGetProportion,
	},
}

var f3SubCmdPowerTableGet = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Get F3 power table at a specific instance ID or latest instance if none is specified.",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("instance", false, true, "instance"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("ec", "Whether to get the power table from EC."),
		cmds.BoolOption("by-tipset", "Gets power table by translating instance into tipset."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		if len(req.Arguments) > 1 {
			return fmt.Errorf("too many arguments")
		}

		ctx := requestContext(req)
		buf := new(bytes.Buffer)

		api := env.(*node.Env).F3API
		progress, err := api.F3GetProgress(ctx)
		if err != nil {
			return fmt.Errorf("getting progress: %w", err)
		}

		var instance uint64
		if len(req.Arguments) > 0 {
			instance, err = strconv.ParseUint(req.Arguments[0], 10, 64)
			if err != nil {
				return fmt.Errorf("parsing instance: %w", err)
			}
			if instance > progress.ID {
				// TODO: Technically we can return power table for instances ahead as long as
				//       instance is within lookback. Implement it.
				return fmt.Errorf("instance is ahead the current instance in progress: %d > %d", instance, progress.ID)
			}
		} else {
			instance = progress.ID
		}

		byTipset, _ := req.Options["by-tipset"].(bool)
		ec, _ := req.Options["ec"].(bool)
		var result = struct {
			Instance   uint64
			FromEC     bool
			ByTipset   bool
			PowerTable struct {
				CID         string
				Entries     gpbft.PowerEntries
				Total       gpbft.StoragePower
				ScaledTotal int64
			}
		}{
			Instance: instance,
			FromEC:   ec,
			ByTipset: byTipset,
		}

		var expectedPowerTableCID cid.Cid
		if !result.ByTipset {
			result.PowerTable.Entries, err = api.F3GetPowerTableByInstance(ctx, instance)
			if err != nil {
				return fmt.Errorf("getting f3 power table at instance %d: %w", instance, err)
			}
		} else {
			var ltsk types.TipSetKey
			ltsk, expectedPowerTableCID, err = f3GetPowerTableTSKByInstance(ctx, api, env.(*node.Env).ChainAPI, instance)
			if err != nil {
				return fmt.Errorf("getting power table tsk for instance %d: %w", instance, err)
			}

			if result.FromEC {
				result.PowerTable.Entries, err = api.F3GetECPowerTable(ctx, ltsk)
			} else {
				result.PowerTable.Entries, err = api.F3GetF3PowerTable(ctx, ltsk)
			}
			if err != nil {
				return fmt.Errorf("getting f3 power table at instance %d: %w", instance, err)
			}
		}

		pt := gpbft.NewPowerTable()
		if err := pt.Add(result.PowerTable.Entries...); err != nil {
			// Sanity check the entries returned by the API.
			return fmt.Errorf("retrieved power table is not valid for instance %d: %w", instance, err)
		}
		result.PowerTable.Total = pt.Total
		result.PowerTable.ScaledTotal = pt.ScaledTotal

		actualPowerTableCID, err := certs.MakePowerTableCID(result.PowerTable.Entries)
		if err != nil {
			return fmt.Errorf("gettingh power table CID at instance %d: %w", instance, err)
		}
		if !cid.Undef.Equals(expectedPowerTableCID) && !expectedPowerTableCID.Equals(actualPowerTableCID) {
			return fmt.Errorf("expected power table CID %s at instance %d, got: %s", expectedPowerTableCID, instance, actualPowerTableCID)
		}
		result.PowerTable.CID = actualPowerTableCID.String()

		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling f3 power table at instance %d: %w", instance, err)
		}
		_, _ = fmt.Fprint(buf, string(output))

		return re.Emit(buf)
	},
}

var f3SubCmdPowerTableGetProportion = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Gets the total proportion of power for a list of actors at a given instance.",
	},
	Arguments: []cmds.Argument{},
	Options: []cmds.Option{
		cmds.StringsOption("actor-id", "actor ids"),
		cmds.BoolOption("ec", "Whether to get the power table from EC."),
		cmds.Uint64Option("instance", "The F3 instance ID."),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		ctx := requestContext(req)
		buf := new(bytes.Buffer)

		api := env.(*node.Env).F3API

		progress, err := api.F3GetProgress(ctx)
		if err != nil {
			return fmt.Errorf("getting progress: %w", err)
		}

		instance, ok := req.Options["instance"].(uint64)
		if ok {
			if instance > progress.ID {
				// TODO: Technically we can return power table for instances ahead as long as
				//       instance is within lookback. Implement it.
				return fmt.Errorf("instance is ahead the current instance in progress: %d > %d", instance, progress.ID)
			}
		} else {
			instance = progress.ID
		}

		ltsk, expectedPowerTableCID, err := f3GetPowerTableTSKByInstance(ctx, api, env.(*node.Env).ChainAPI, instance)
		if err != nil {
			return fmt.Errorf("getting power table tsk for instance %d: %w", instance, err)
		}

		ec, _ := req.Options["ec"].(bool)
		var result = struct {
			Instance   uint64
			FromEC     bool
			PowerTable struct {
				CID         string
				ScaledTotal int64
			}
			ScaledSum  int64
			Proportion float64
			NotFound   []gpbft.ActorID
		}{
			Instance: instance,
			FromEC:   ec,
		}

		var powerEntries gpbft.PowerEntries
		if result.FromEC {
			powerEntries, err = api.F3GetECPowerTable(ctx, ltsk)
		} else {
			powerEntries, err = api.F3GetF3PowerTable(ctx, ltsk)
		}
		if err != nil {
			return fmt.Errorf("getting f3 power table at instance %d: %w", instance, err)
		}

		actualPowerTableCID, err := certs.MakePowerTableCID(powerEntries)
		if err != nil {
			return fmt.Errorf("gettingh power table CID at instance %d: %w", instance, err)
		}
		if !cid.Undef.Equals(expectedPowerTableCID) && !expectedPowerTableCID.Equals(actualPowerTableCID) {
			return fmt.Errorf("expected power table CID %s at instance %d, got: %s", expectedPowerTableCID, instance, actualPowerTableCID)
		}
		result.PowerTable.CID = actualPowerTableCID.String()

		pt := gpbft.NewPowerTable()
		if err := pt.Add(powerEntries...); err != nil {
			return fmt.Errorf("constructing power table from entries: %w", err)
		}
		result.PowerTable.ScaledTotal = pt.ScaledTotal

		inputActorIDs, _ := req.Options["actor-id"].([]string)
		seenIDs := map[gpbft.ActorID]struct{}{}
		for _, stringID := range inputActorIDs {
			var actorID gpbft.ActorID
			switch addr, err := address.NewFromString(stringID); {
			case err == nil:
				idAddr, err := address.IDFromAddress(addr)
				if err != nil {
					return fmt.Errorf("parsing ID from address %q: %w", stringID, err)
				}
				actorID = gpbft.ActorID(idAddr)
			case errors.Is(err, address.ErrUnknownNetwork),
				errors.Is(err, address.ErrUnknownProtocol):
				// Try parsing as uint64 straight up.
				id, err := strconv.ParseUint(stringID, 10, 64)
				if err != nil {
					return fmt.Errorf("parsing as uint64 %q: %w", stringID, err)
				}
				actorID = gpbft.ActorID(id)
			default:
				return fmt.Errorf("parsing address %q: %w", stringID, err)
			}
			// Prune duplicate IDs.
			if _, ok := seenIDs[actorID]; ok {
				continue
			}
			seenIDs[actorID] = struct{}{}
			scaled, key := pt.Get(actorID)
			if key == nil {
				result.NotFound = append(result.NotFound, actorID)
				continue
			}
			result.ScaledSum += scaled
		}
		result.Proportion = float64(result.ScaledSum) / float64(result.PowerTable.ScaledTotal)
		output, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			return fmt.Errorf("marshalling f3 power table at instance %d: %w", instance, err)
		}
		_, _ = fmt.Fprint(buf, string(output))

		return re.Emit(buf)
	},
}

func f3GetPowerTableTSKByInstance(ctx context.Context, f3api v1api.IF3, chainAPI v1api.IChain, instance uint64) (types.TipSetKey, cid.Cid, error) {
	mfst, err := f3api.F3GetManifest(ctx)
	if err != nil {
		return types.EmptyTSK, cid.Undef, fmt.Errorf("getting manifest: %w", err)
	}

	if instance < mfst.InitialInstance+mfst.CommitteeLookback {
		ts, err := chainAPI.ChainGetTipSetByHeight(ctx, abi.ChainEpoch(mfst.BootstrapEpoch-mfst.EC.Finality), types.EmptyTSK)
		if err != nil {
			return types.EmptyTSK, cid.Undef, fmt.Errorf("getting bootstrap epoch tipset: %w", err)
		}
		return ts.Key(), mfst.InitialPowerTable, nil
	}

	previous, err := f3api.F3GetCertificate(ctx, instance-1)
	if err != nil {
		return types.EmptyTSK, cid.Undef, fmt.Errorf("getting certificate for previous instance: %w", err)
	}
	lookback, err := f3api.F3GetCertificate(ctx, instance-mfst.CommitteeLookback)
	if err != nil {
		return types.EmptyTSK, cid.Undef, fmt.Errorf("getting certificate for lookback instance: %w", err)
	}
	ltsk, err := types.TipSetKeyFromBytes(lookback.ECChain.Head().Key)
	if err != nil {
		return types.EmptyTSK, cid.Undef, fmt.Errorf("getting lotus tipset key from head of lookback certificate: %w", err)
	}
	return ltsk, previous.SupplementalData.PowerTable, nil
}

func prettyPrintManifest(out io.Writer, manifest *manifest.Manifest) error {
	if manifest == nil {
		_, err := fmt.Fprintln(out, "Manifest: None")
		return err
	}

	return f3Templates.ExecuteTemplate(out, "f3_manifest.go.tmpl", manifest)
}
