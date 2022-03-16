package networks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"

	"github.com/filecoin-project/venus/fixtures/asset"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/pkg/genesis"
	"github.com/filecoin-project/venus/pkg/repo"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
	"github.com/filecoin-project/venus/venus-shared/types"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
)

func GetNetworkFromName(name string) (constants.NetworkType, error) {
	switch name {
	case "mainnet":
		return constants.NetworkMainnet, nil
	case "force":
		return constants.NetworkForce, nil
	case "integrationnet":
		return constants.Integrationnet, nil
	case "2k":
		return constants.Network2k, nil
	case "cali":
		return constants.NetworkCalibnet, nil
	case "interop":
		return constants.NetworkInterop, nil
	case "butterfly":
		return constants.NetworkButterfly, nil
	default:
		return 0, fmt.Errorf("unknown network name %s", name)
	}
}

func SetConfigFromOptions(cfg *config.Config, network string) error {
	// Setup specific config options.
	networkType, err := GetNetworkFromName(network)
	if err != nil {
		return err
	}
	var netcfg *NetworkConf
	switch networkType {
	case constants.NetworkMainnet:
		netcfg = Mainnet()
	case constants.NetworkForce:
		netcfg = ForceNet()
	case constants.Integrationnet:
		netcfg = IntegrationNet()
	case constants.Network2k:
		netcfg = Net2k()
	case constants.NetworkCalibnet:
		netcfg = Calibration()
	case constants.NetworkInterop:
		netcfg = InteropNet()
	case constants.NetworkButterfly:
		netcfg = ButterflySnapNet()
	default:
		return fmt.Errorf("unknown network name %s", network)
	}

	if netcfg != nil {
		cfg.Bootstrap = &netcfg.Bootstrap
		cfg.NetworkParams = &netcfg.Network
	}

	return nil
}

func LoadGenesis(ctx context.Context, rep repo.Repo, sourceName string, network string) (genesis.InitFunc, error) {
	var (
		source io.ReadCloser
		err    error
	)

	if sourceName == "" {
		networkType, err := GetNetworkFromName(network)
		if err != nil {
			return nil, err
		}

		var bs []byte
		switch networkType {
		case constants.NetworkNerpa:
			bs, err = asset.Asset("fixtures/_assets/car/nerpanet.car")
		case constants.NetworkCalibnet:
			bs, err = asset.Asset("fixtures/_assets/car/calibnet.car")
		case constants.NetworkInterop:
			bs, err = asset.Asset("fixtures/_assets/car/interopnet.car")
		case constants.NetworkForce:
			bs, err = asset.Asset("fixtures/_assets/car/forcenet.car")
		case constants.NetworkButterfly:
			bs, err = asset.Asset("fixtures/_assets/car/butterflynet.car")
		default:
			bs, err = asset.Asset("fixtures/_assets/car/mainnet.car")
		}
		if err != nil {
			return gengen.MakeGenesisFunc(), nil
		}
		source = ioutil.NopCloser(bytes.NewReader(bs))
		// return gengen.MakeGenesisFunc(), nil
	} else {
		source, err = openGenesisSource(sourceName)
		if err != nil {
			return nil, err
		}
	}

	defer func() { _ = source.Close() }()

	genesisBlk, err := extractGenesisBlock(ctx, source, rep)
	if err != nil {
		return nil, err
	}

	gif := func(cst cbor.IpldStore, bs blockstore.Blockstore) (*types.BlockHeader, error) {
		return genesisBlk, err
	}

	return gif, nil
}

func extractGenesisBlock(ctx context.Context, source io.ReadCloser, rep repo.Repo) (*types.BlockHeader, error) {
	bs := rep.Datastore()
	ch, err := car.LoadCar(ctx, bs, source)
	if err != nil {
		return nil, err
	}

	// need to check if we are being handed a car file with a single genesis block or an entire chain.
	bsBlk, err := bs.Get(ctx, ch.Roots[0])
	if err != nil {
		return nil, err
	}
	cur, err := types.DecodeBlock(bsBlk.RawData())
	if err != nil {
		return nil, err
	}

	return cur, nil
}

func openGenesisSource(sourceName string) (io.ReadCloser, error) {
	sourceURL, err := url.Parse(sourceName)
	if err != nil {
		return nil, fmt.Errorf("invalid filepath or URL for genesis file: %s", sourceURL)
	}
	var source io.ReadCloser
	if sourceURL.Scheme == "http" || sourceURL.Scheme == "https" {
		// NOTE: This code is temporary. It allows downloading a genesis block via HTTP(S) to be able to join a
		// recently deployed staging devnet.
		response, err := http.Get(sourceName)
		if err != nil {
			return nil, err
		}
		source = response.Body
	} else if sourceURL.Scheme != "" {
		return nil, fmt.Errorf("unsupported protocol for genesis file: %s", sourceURL.Scheme)
	} else {
		file, err := os.Open(sourceName)
		if err != nil {
			return nil, err
		}
		source = file
	}
	return source, nil
}
