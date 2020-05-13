package commands

import (
	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/ipfs/go-cid"
	cmdkit "github.com/ipfs/go-ipfs-cmdkit"
	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
	xerrors "github.com/pkg/errors"
)

var logRetrieval = logging.Logger("commands/retrieval")

var retrievalClientCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Manage retrieval client operations",
	},
	Subcommands: map[string]*cmds.Command{
		"retrieve-piece": clientRetrievePieceCmd,
	},
}

var clientRetrievePieceCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Read out piece data stored by a miner on the network",
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("miner", true, false, "Retrieval miner actor address"),
		cmdkit.StringArg("cid", true, false, "Content identifier of piece to read"),
	},
	Run: func(req *cmds.Request, re cmds.ResponseEmitter, env cmds.Environment) error {
		porcelainAPI := GetPorcelainAPI(env)

		minerAddr, err := address.NewFromString(req.Arguments[0])
		if err != nil {
			return err
		}

		payloadCID, err := cid.Decode(req.Arguments[1])
		if err != nil {
			return err
		}

		// first see if we have it locally
		rdr, err := porcelainAPI.DAGCat(req.Context, payloadCID)
		if err == nil {
			return re.Emit(rdr)
		}

		logRetrieval.Infof("payloadCID %s not found locally: %w", payloadCID.String(), err)

		head := porcelainAPI.ChainHeadKey()
		status, err := porcelainAPI.MinerGetStatus(req.Context, minerAddr, head)
		if err != nil {
			return err
		}

		client := GetRetrievalAPI(env).Client()

		retPeer := retrievalmarket.RetrievalPeer{Address: minerAddr, ID: status.PeerID}
		resp, err := client.Query(req.Context, retPeer, payloadCID, retrievalmarket.QueryParams{})
		if err != nil {
			return err
		}

		retParams := retrievalmarket.NewParamsV0(resp.MinPricePerByte, resp.MaxPaymentInterval, resp.MaxPaymentIntervalIncrease)
		clientWallet, err := porcelainAPI.WalletDefaultAddress()
		if err != nil {
			return err
		}
		retrievalPrice := resp.PieceRetrievalPrice()
		bal, err := porcelainAPI.WalletBalance(req.Context, clientWallet)
		if err != nil {
			return err
		}

		if bal.LessThan(retrievalPrice) {
			return xerrors.New("insufficient wallet balance for retrieval")
		}

		retrievalResult := make(chan error, 1)
		unsub := client.SubscribeToEvents(func(event retrievalmarket.ClientEvent, state retrievalmarket.ClientDealState) {
			if state.PayloadCID.Equals(payloadCID) {
				switch state.Status {
				case retrievalmarket.DealStatusFailed, retrievalmarket.DealStatusErrored:
					retrievalResult <- xerrors.Errorf("Retrieval Error: %s", state.Message)
				case retrievalmarket.DealStatusCompleted:
					retrievalResult <- nil
				}
			}
		})

		readCloser, err := client.Retrieve(req.Context,
			payloadCID,
			retParams,
			retrievalPrice,
			status.PeerID,
			clientWallet,
			minerAddr)
		if err != nil {
			return err
		}
		select {
		case <-req.Context.Done():
			return xerrors.New("Retrieval Timed Out")
		case err := <-retrievalResult:
			if err != nil {
				return xerrors.Errorf("RetrieveUnixfs: %w", err)
			}
		}
		unsub()
		return re.Emit(readCloser)
	},
}
