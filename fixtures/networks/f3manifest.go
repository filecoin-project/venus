package networks

import (
	"embed"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/filecoin-project/go-f3/manifest"
	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/ipfs/go-cid"
)

//go:embed f3manifest
var f3FS embed.FS

func getF3ManifestBytes(networkType types.NetworkType) ([]byte, error) {
	fileName := ""
	switch networkType {
	case types.NetworkForce, types.Network2k:
		fileName = "f3manifest_2k.json"
	case types.NetworkInterop:
		fileName = "f3manifest_interop.json"
	case types.NetworkButterfly:
		fileName = "f3manifest_butterfly.json"
	case types.NetworkCalibnet:
		fileName = "f3manifest_calibnet.json"
	default:
		fileName = "f3manifest_mainnet.json"
	}

	file, err := f3FS.Open(filepath.Join("f3manifest", fileName))
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint

	return io.ReadAll(file)
}

func F3Manifest(nt types.NetworkType, blockDelaySecs int) *manifest.Manifest {
	data, err := getF3ManifestBytes(nt)
	if err != nil {
		log.Panicf("failed to get f3 manifest bytes")
	}

	var manif manifest.Manifest
	if err := json.Unmarshal(data, &manif); err != nil {
		log.Panicf("failed to unmarshal F3 manifest: %s", err)
	}
	if err := manif.Validate(); err != nil {
		log.Panicf("invalid F3 manifest: %s", err)
	}

	if ptCid := os.Getenv("F3_INITIAL_POWERTABLE_CID"); ptCid != "" {
		if k, err := cid.Parse(ptCid); err != nil {
			log.Errorf("failed to parse F3_INITIAL_POWERTABLE_CID %q: %s", ptCid, err)
		} else if manif.InitialPowerTable.Defined() && k != manif.InitialPowerTable {
			log.Errorf("ignoring F3_INITIAL_POWERTABLE_CID as lotus has a hard-coded initial F3 power table")
		} else {
			manif.InitialPowerTable = k
		}
	}
	if !manif.InitialPowerTable.Defined() {
		log.Warn("initial power table is not specified, it will be populated automatically assuming this is testing network")
	}

	// EC Period sanity check
	if manif.EC.Period != time.Duration(blockDelaySecs)*time.Second {
		log.Panicf("static manifest EC period is %v, expected %v", manif.EC.Period, time.Duration(blockDelaySecs)*time.Second)
	}
	return &manif
}
