package api

import (
	"encoding/json"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
)

// IDDetails is a collection of information about a node.
// TODO: do we want to use strings here, or concrete types?
type IDDetails struct {
	Addresses       []string
	ID              peer.ID
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

// MarshalJSON implements json.Marshaler
func (idd IDDetails) MarshalJSON() ([]byte, error) {
	v := map[string]interface{}{
		"Addresses":       idd.Addresses,
		"ID":              idd.ID.Pretty(),
		"AgentVersion":    idd.AgentVersion,
		"ProtocolVersion": idd.ProtocolVersion,
		"PublicKey":       idd.PublicKey,
	}
	return json.Marshal(v)
}

// UnmarshalJSON implements Unmarshaler
func (idd *IDDetails) UnmarshalJSON(data []byte) error {
	var v map[string]*json.RawMessage
	var err error
	if err = json.Unmarshal(data, &v); err != nil {
		return err
	}

	if err = json.Unmarshal(*v["Addresses"], &idd.Addresses); err != nil {
		return err
	}

	var id string
	if err = json.Unmarshal(*v["ID"], &id); err != nil {
		return err
	}
	if idd.ID, err = peer.IDB58Decode(id); err != nil {
		return err
	}

	if err = json.Unmarshal(*v["AgentVersion"], &idd.AgentVersion); err != nil {
		return err
	}
	if err = json.Unmarshal(*v["ProtocolVersion"], &idd.ProtocolVersion); err != nil {
		return err
	}
	if err = json.Unmarshal(*v["PublicKey"], &idd.PublicKey); err != nil {
		return err
	}
	return nil
}
