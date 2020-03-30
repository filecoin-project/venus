package retrieval

import (
	iface "github.com/filecoin-project/go-fil-markets/retrievalmarket"
)

// API is the retrieval api for the test environment
type API interface {
	Client() iface.RetrievalClient
	Provider() iface.RetrievalProvider
}
