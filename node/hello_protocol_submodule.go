package node

import "github.com/filecoin-project/go-filecoin/protocol/hello"

// HelloProtocolSubmodule enhances the `Node` with "Hello" protocol capabilities.
type HelloProtocolSubmodule struct {
	HelloSvc *hello.Handler
}
