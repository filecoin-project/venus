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

	"github.com/filecoin-project/venus/fixtures/assets"
	"github.com/filecoin-project/venus/pkg/config"
	"github.com/filecoin-project/venus/pkg/genesis"
	"github.com/filecoin-project/venus/pkg/repo"
	"github.com/filecoin-project/venus/pkg/util/blockstoreutil"
	gengen "github.com/filecoin-project/venus/tools/gengen/util"
	"github.com/filecoin-project/venus/venus-shared/types"
	cbor "github.com/ipfs/go-ipld-cbor"
	"github.com/ipld/go-car"
)

func GetNetworkFromName(name string) (types.NetworkType, error) {
	switch name {
	case "mainnet":
		return types.NetworkMainnet, nil
	case "force":
		return types.NetworkForce, nil
	case "integrationnet":
		return types.Integrationnet, nil
	case "2k":
		return types.Network2k, nil
	case "cali":
		return types.NetworkCalibnet, nil
	case "interop":
		return types.NetworkInterop, nil
	case "butterfly":
		return types.NetworkButterfly, nil
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
	case types.NetworkMainnet:
		netcfg = Mainnet()
	case types.NetworkForce:
		netcfg = ForceNet()
	case types.Integrationnet:
		netcfg = IntegrationNet()
	case types.Network2k:
		netcfg = Net2k()
	case types.NetworkCalibnet:
		netcfg = Calibration()
	case types.NetworkInterop:
		netcfg = InteropNet()
	case types.NetworkButterfly:
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

		bs, err := assets.GetGenesis(networkType)
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

	gif := func(cst cbor.IpldStore, bs blockstoreutil.Blockstore) (*types.BlockHeader, error) {
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
