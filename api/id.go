package api

// IDDetails is a collection of information about a node.
// TODO: do we want to use strings here, or concrete types?
type IDDetails struct {
	Addresses       []string
	ID              string
	AgentVersion    string
	ProtocolVersion string
	PublicKey       string
}

// ID is the interface that defines methods to fetch identifying information
// about the underlying node.
type ID interface {
	// Details, returns detailed information about the underlying node.
	Details() (*IDDetails, error)
}
