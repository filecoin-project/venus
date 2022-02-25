package client

import (
	"context"

	"github.com/filecoin-project/go-address"
	datatransfer "github.com/filecoin-project/go-data-transfer"
	"github.com/filecoin-project/go-fil-markets/retrievalmarket"
	"github.com/filecoin-project/go-fil-markets/storagemarket"
	"github.com/ipfs/go-cid"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/filecoin-project/venus/venus-shared/types"
	"github.com/filecoin-project/venus/venus-shared/types/market"
	"github.com/filecoin-project/venus/venus-shared/types/market/client"
)

type IMarketClient interface {
	// ClientImport imports file under the specified path into filestore.
	ClientImport(ctx context.Context, ref client.FileRef) (*client.ImportRes, error) //perm:admin
	// ClientRemoveImport removes file import
	ClientRemoveImport(ctx context.Context, importID client.ImportID) error //perm:admin
	// ClientStartDeal proposes a deal with a miner.
	ClientStartDeal(ctx context.Context, params *client.StartDealParams) (*cid.Cid, error) //perm:admin
	// ClientStatelessDeal fire-and-forget-proposes an offline deal to a miner without subsequent tracking.
	ClientStatelessDeal(ctx context.Context, params *client.StartDealParams) (*cid.Cid, error) //perm:write
	// ClientGetDealInfo returns the latest information about a given deal.
	ClientGetDealInfo(context.Context, cid.Cid) (*client.DealInfo, error) //perm:read
	// ClientListDeals returns information about the deals made by the local client.
	ClientListDeals(ctx context.Context) ([]client.DealInfo, error) //perm:write
	// ClientGetDealUpdates returns the status of updated deals
	ClientGetDealUpdates(ctx context.Context) (<-chan client.DealInfo, error) //perm:write
	// ClientGetDealStatus returns status given a code
	ClientGetDealStatus(ctx context.Context, statusCode uint64) (string, error) //perm:read
	// ClientHasLocal indicates whether a certain CID is locally stored.
	ClientHasLocal(ctx context.Context, root cid.Cid) (bool, error) //perm:write
	// ClientFindData identifies peers that have a certain file, and returns QueryOffers (one per peer).
	ClientFindData(ctx context.Context, root cid.Cid, piece *cid.Cid) ([]client.QueryOffer, error) //perm:read
	// ClientMinerQueryOffer returns a QueryOffer for the specific miner and file.
	ClientMinerQueryOffer(ctx context.Context, miner address.Address, root cid.Cid, piece *cid.Cid) (client.QueryOffer, error) //perm:read
	// ClientRetrieve initiates the retrieval of a file, as specified in the order.
	ClientRetrieve(ctx context.Context, params client.RetrievalOrder) (*client.RestrievalRes, error) //perm:admin
	// ClientRetrieveWait waits for retrieval to be complete
	ClientRetrieveWait(ctx context.Context, deal retrievalmarket.DealID) error //perm:admin
	// ClientExport exports a file stored in the local filestore to a system file
	ClientExport(ctx context.Context, exportRef client.ExportRef, fileRef client.FileRef) error //perm:admin
	ClientListRetrievals(ctx context.Context) ([]client.RetrievalInfo, error)                   //perm:write
	// ClientGetRetrievalUpdates returns status of updated retrieval deals
	ClientGetRetrievalUpdates(ctx context.Context) (<-chan client.RetrievalInfo, error) //perm:write
	// ClientQueryAsk returns a signed StorageAsk from the specified miner.
	ClientQueryAsk(ctx context.Context, p peer.ID, miner address.Address) (*storagemarket.StorageAsk, error) //perm:read
	// ClientCalcCommP calculates the CommP and data size of the specified CID
	ClientDealPieceCID(ctx context.Context, root cid.Cid) (client.DataCIDSize, error) //perm:read
	// ClientCalcCommP calculates the CommP for a specified file
	ClientCalcCommP(ctx context.Context, inpath string) (*client.CommPRet, error) //perm:write
	// ClientGenCar generates a CAR file for the specified file.
	ClientGenCar(ctx context.Context, ref client.FileRef, outpath string) error //perm:write
	// ClientDealSize calculates real deal data size
	ClientDealSize(ctx context.Context, root cid.Cid) (client.DataSize, error) //perm:read
	// ClientListTransfers returns the status of all ongoing transfers of data
	ClientListDataTransfers(ctx context.Context) ([]market.DataTransferChannel, error)        //perm:write
	ClientDataTransferUpdates(ctx context.Context) (<-chan market.DataTransferChannel, error) //perm:write
	// ClientRestartDataTransfer attempts to restart a data transfer with the given transfer ID and other peer
	ClientRestartDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	// ClientCancelDataTransfer cancels a data transfer with the given transfer ID and other peer
	ClientCancelDataTransfer(ctx context.Context, transferID datatransfer.TransferID, otherPeer peer.ID, isInitiator bool) error //perm:write
	// ClientRetrieveTryRestartInsufficientFunds attempts to restart stalled retrievals on a given payment channel
	// which are stuck due to insufficient funds
	ClientRetrieveTryRestartInsufficientFunds(ctx context.Context, paymentChannel address.Address) error //perm:write

	// ClientCancelRetrievalDeal cancels an ongoing retrieval deal based on DealID
	ClientCancelRetrievalDeal(ctx context.Context, dealid retrievalmarket.DealID) error //perm:write

	// ClientUnimport removes references to the specified file from filestore
	//ClientUnimport(path string)

	// ClientListImports lists imported files and their root CIDs
	ClientListImports(ctx context.Context) ([]client.Import, error) //perm:write
	DefaultAddress(ctx context.Context) (address.Address, error)    //perm:read

	MarketAddBalance(ctx context.Context, wallet, addr address.Address, amt types.BigInt) (cid.Cid, error)                   //perm:write
	MarketGetReserved(ctx context.Context, addr address.Address) (types.BigInt, error)                                       //perm:read
	MarketReserveFunds(ctx context.Context, wallet address.Address, addr address.Address, amt types.BigInt) (cid.Cid, error) //perm:write
	MarketReleaseFunds(ctx context.Context, addr address.Address, amt types.BigInt) error                                    //perm:write
	MarketWithdraw(ctx context.Context, wallet, addr address.Address, amt types.BigInt) (cid.Cid, error)                     //perm:write

	MessagerWaitMessage(ctx context.Context, mid cid.Cid) (*types.MsgLookup, error)                            //perm:read
	MessagerPushMessage(ctx context.Context, msg *types.Message, meta *types.MessageSendSpec) (cid.Cid, error) //perm:write
	MessagerGetMessage(ctx context.Context, mid cid.Cid) (*types.Message, error)                               //perm:read
}
