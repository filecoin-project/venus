package block

type BeaconEntry struct {
	_     struct{} `cbor:",toarray"`
	Round uint64
	Data  []byte
}
