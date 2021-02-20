package remotewallet

import (
	"context"
	"github.com/filecoin-project/go-jsonrpc"
	"github.com/ipfs-force-community/venus-wallet/storage"
	"github.com/ipfs-force-community/venus-wallet/api"
	"net/http"
)



func SetupRemoteWallet() func
