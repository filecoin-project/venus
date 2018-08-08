package api

// IdDetails is a collection of information about a node.
// TODO: do we want to use strings here, or concrete types?
type IdDetails struct {
	Addresses       []string
	ID              string
	AgentVersion    string
	ProtocolVersion string
	PublicKey       string
}

type Id interface {
	// Details, returns detailed information about the underlying node.
	Details() (*IdDetails, error)
}
