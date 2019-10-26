package node

import "github.com/filecoin-project/go-filecoin/internal/pkg/discovery"

// HelloProtocolSubmodule enhances the `Node` with "Hello" protocol capabilities.
type HelloProtocolSubmodule struct {
	Handler *discovery.HelloProtocolHandler
}
