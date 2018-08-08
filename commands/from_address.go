package commands

import (
	"gx/ipfs/QmVmDhyTTUcQXFD1rRQ64fGLMSAoaQvNH3hwuaCFAPq2hy/errors"
	cmdkit "gx/ipfs/QmdE4gMduCKCGAcczM2F5ioYDfdeKuPix138wrES1YSr7f/go-ipfs-cmdkit"

	"github.com/filecoin-project/go-filecoin/api_impl"
	"github.com/filecoin-project/go-filecoin/node"
	"github.com/filecoin-project/go-filecoin/types"
)

func fromAddress(opts cmdkit.OptMap, nd *node.Node) (ret types.Address, err error) {
	o := opts["from"]
	if o != nil {
		ret, err = types.NewAddressFromString(o.(string))
		if err != nil {
			err = errors.Wrap(err, "invalid from address")
		}
	} else {
		ret, err = nd.DefaultSenderAddress()
		if (err != nil && err != node.ErrNoDefaultMessageFromAddress) || ret != (types.Address{}) {
			return
		}

		err = api_impl.ErrCouldNotDefaultFromAddress
	}
	return
}
