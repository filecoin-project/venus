package types

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	mathbig "math/big"
	"strconv"
	"strings"

	"github.com/ipfs/go-cid"
	mh "github.com/multiformats/go-multihash"
	"github.com/multiformats/go-varint"
	"golang.org/x/crypto/sha3"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	"github.com/filecoin-project/go-state-types/big"
	builtintypes "github.com/filecoin-project/go-state-types/builtin"
	"github.com/filecoin-project/venus/pkg/constants"
	"github.com/filecoin-project/venus/venus-shared/actors"
)

var expectedHashPrefix = cid.Prefix{
	Version:  1,
	Codec:    cid.DagCBOR,
	MhType:   uint64(mh.BLAKE2B_MIN + 31),
	MhLength: 32,
}.Bytes()

var ErrInvalidAddress = errors.New("invalid Filecoin Eth address")

// Research into Filecoin chain behaviour suggests that probabilistic finality
// generally approaches the intended stability guarantee at, or near, 30 epochs.
// Although a strictly "finalized" safe recommendation remains 900 epochs.
// See https://github.com/filecoin-project/FIPs/blob/master/FRCs/frc-0089.md
const SafeEpochDelay = abi.ChainEpoch(30)

// mainnet
var Eip155ChainID = 314

func SetEip155ChainID(val int) {
	Eip155ChainID = val
	initEthLegacy155TxSignature()
}

type EthUint64 uint64

func (e EthUint64) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.Hex())
}

// UnmarshalJSON should be able to parse these types of input:
// 1. a JSON string containing a hex-encoded uint64 starting with 0x
// 2. a JSON string containing an uint64 in decimal
// 3. a string containing an uint64 in decimal
func (e *EthUint64) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		base := 10
		if strings.HasPrefix(s, "0x") {
			base = 16
		}
		parsedInt, err := strconv.ParseUint(strings.Replace(s, "0x", "", -1), base, 64)
		if err != nil {
			return err
		}
		eint := EthUint64(parsedInt)
		*e = eint
		return nil
	} else if eint, err := strconv.ParseUint(string(b), 10, 64); err == nil {
		*e = EthUint64(eint)
		return nil
	}
	return fmt.Errorf("cannot interpret %s as a hex-encoded uint64, or a number", string(b))
}

func EthUint64FromHex(s string) (EthUint64, error) {
	parsedInt, err := strconv.ParseUint(strings.Replace(s, "0x", "", -1), 16, 64)
	if err != nil {
		return EthUint64(0), err
	}
	return EthUint64(parsedInt), nil
}

// Parse a uint64 from big-endian encoded bytes.
func EthUint64FromBytes(b []byte) (EthUint64, error) {
	if len(b) != 32 {
		return 0, fmt.Errorf("eth int must be 32 bytes long")
	}
	var zeros [32 - 8]byte
	if !bytes.Equal(b[:len(zeros)], zeros[:]) {
		return 0, fmt.Errorf("eth int overflows 64 bits")
	}
	return EthUint64(binary.BigEndian.Uint64(b[len(zeros):])), nil
}

func (e EthUint64) Hex() string {
	if e == 0 {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", e)
}

// EthBigInt represents a large integer whose zero value serializes to "0x0".
type EthBigInt big.Int

var (
	EthBigIntZero = EthBigInt{Int: big.Zero().Int}
)

func (e EthBigInt) String() string {
	if e.Int == nil || e.Int.BitLen() == 0 {
		return "0x0"
	}
	return fmt.Sprintf("0x%x", e.Int)
}

func (e EthBigInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e *EthBigInt) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	replaced := strings.Replace(s, "0x", "", -1)
	if len(replaced)%2 == 1 {
		replaced = "0" + replaced
	}

	i := new(mathbig.Int)
	i.SetString(replaced, 16)

	*e = EthBigInt(big.NewFromGo(i))
	return nil
}

// EthBytes represent arbitrary bytes. A nil or empty slice serializes to "0x".
type EthBytes []byte

func (e EthBytes) String() string {
	if len(e) == 0 {
		return "0x"
	}
	return "0x" + hex.EncodeToString(e)
}

func (e EthBytes) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.String())
}

func (e *EthBytes) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	s = strings.Replace(s, "0x", "", -1)
	if len(s)%2 == 1 {
		s = "0" + s
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}

	*e = decoded
	return nil
}

type EthBlock struct {
	Hash             EthHash    `json:"hash"`
	ParentHash       EthHash    `json:"parentHash"`
	Sha3Uncles       EthHash    `json:"sha3Uncles"`
	Miner            EthAddress `json:"miner"`
	StateRoot        EthHash    `json:"stateRoot"`
	TransactionsRoot EthHash    `json:"transactionsRoot"`
	ReceiptsRoot     EthHash    `json:"receiptsRoot"`
	LogsBloom        EthBytes   `json:"logsBloom"`
	Difficulty       EthUint64  `json:"difficulty"`
	TotalDifficulty  EthUint64  `json:"totalDifficulty"`
	Number           EthUint64  `json:"number"`
	GasLimit         EthUint64  `json:"gasLimit"`
	GasUsed          EthUint64  `json:"gasUsed"`
	Timestamp        EthUint64  `json:"timestamp"`
	Extradata        EthBytes   `json:"extraData"`
	MixHash          EthHash    `json:"mixHash"`
	Nonce            EthNonce   `json:"nonce"`
	BaseFeePerGas    EthBigInt  `json:"baseFeePerGas"`
	Size             EthUint64  `json:"size"`
	// can be []EthTx or []string depending on query params
	Transactions []interface{} `json:"transactions"`
	Uncles       []EthHash     `json:"uncles"`
}

const EthBloomSize = 2048

var (
	EmptyEthBloom  = [EthBloomSize / 8]byte{}
	FullEthBloom   = [EthBloomSize / 8]byte{}
	EmptyEthHash   = EthHash{}
	EmptyUncleHash = One(ParseEthHash("0x1dcc4de8dec75d7aab85b567b6ccd41ad312451b948a7413f0a142fd40d49347")) // Keccak-256 of an RLP of an empty array
	EmptyRootHash  = One(ParseEthHash("0x56e81f171bcc55a6ff8345e692c0f86e5b48e01b996cadc001622fb5e363b421")) // Keccak-256 hash of the RLP of null
	EmptyEthInt    = EthUint64(0)
	EmptyEthNonce  = [8]byte{0, 0, 0, 0, 0, 0, 0, 0}
)

func init() {
	for i := range FullEthBloom {
		FullEthBloom[i] = 0xff
	}
}

func NewEthBlock(hasTransactions bool, tipsetLen int) EthBlock {
	b := EthBlock{
		Sha3Uncles:       EmptyUncleHash, // Sha3Uncles set to a hardcoded value which is used by some clients to determine if has no uncles.,
		StateRoot:        EmptyEthHash,
		TransactionsRoot: EmptyRootHash, // TransactionsRoot set to a hardcoded value which is used by some clients to determine if has no transactions.,
		ReceiptsRoot:     EmptyEthHash,
		Difficulty:       EmptyEthInt,
		LogsBloom:        FullEthBloom[:],
		Extradata:        []byte{},
		MixHash:          EmptyEthHash,
		Nonce:            EmptyEthNonce,
		GasLimit:         EthUint64(constants.BlockGasLimit * uint64(tipsetLen)),
		Uncles:           []EthHash{},
		Transactions:     []interface{}{},
	}
	if hasTransactions {
		b.TransactionsRoot = EmptyEthHash
	}

	return b
}

type EthCall struct {
	From     *EthAddress `json:"from"`
	To       *EthAddress `json:"to"`
	Gas      EthUint64   `json:"gas"`
	GasPrice EthBigInt   `json:"gasPrice"`
	Value    EthBigInt   `json:"value"`
	Data     EthBytes    `json:"data"`
}

func (c *EthCall) ToFilecoinMessage() (*Message, error) {
	var from address.Address
	if c.From == nil || *c.From == (EthAddress{}) {
		// Send from the filecoin "system" address.
		var err error
		from, err = (EthAddress{}).ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to construct the ethereum system address: %w", err)
		}
	} else {
		// The from address must be translatable to an f4 address.
		var err error
		from, err = c.From.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("failed to translate sender address (%s): %w", c.From.String(), err)
		}
		if p := from.Protocol(); p != address.Delegated {
			return nil, fmt.Errorf("expected a class 4 address, got: %d: %w", p, err)
		}
	}

	var params []byte
	if len(c.Data) > 0 {
		initcode := abi.CborBytes(c.Data)
		params2, err := actors.SerializeParams(&initcode)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize params: %w", err)
		}
		params = params2
	}

	var to address.Address
	var method abi.MethodNum
	if c.To == nil {
		// this is a contract creation
		to = builtintypes.EthereumAddressManagerActorAddr
		method = builtintypes.MethodsEAM.CreateExternal
	} else {
		addr, err := c.To.ToFilecoinAddress()
		if err != nil {
			return nil, fmt.Errorf("cannot get Filecoin address: %w", err)
		}
		to = addr
		method = builtintypes.MethodsEVM.InvokeContract
	}

	return &Message{
		From:       from,
		To:         to,
		Value:      big.Int(c.Value),
		Method:     method,
		Params:     params,
		GasLimit:   constants.BlockGasLimit,
		GasFeeCap:  big.Zero(),
		GasPremium: big.Zero(),
	}, nil
}

func (c *EthCall) UnmarshalJSON(b []byte) error {
	type TempEthCall EthCall
	var params TempEthCall

	if err := json.Unmarshal(b, &params); err != nil {
		return err
	}
	*c = EthCall(params)
	return nil
}

type EthSyncingResult struct {
	DoneSync      bool
	StartingBlock EthUint64
	CurrentBlock  EthUint64
	HighestBlock  EthUint64
}

func (sr EthSyncingResult) MarshalJSON() ([]byte, error) {
	if sr.DoneSync {
		// when done syncing, the json response should be '"result": false'
		return []byte("false"), nil
	}

	// need to do an anonymous struct to avoid infinite recursion
	return json.Marshal(&struct {
		StartingBlock EthUint64 `json:"startingblock"`
		CurrentBlock  EthUint64 `json:"currentblock"`
		HighestBlock  EthUint64 `json:"highestblock"`
	}{
		StartingBlock: sr.StartingBlock,
		CurrentBlock:  sr.CurrentBlock,
		HighestBlock:  sr.HighestBlock})
}

const (
	EthAddressLength = 20
	EthHashLength    = 32
)

type EthNonce [8]byte

func (n EthNonce) String() string {
	return "0x" + hex.EncodeToString(n[:])
}

func (n EthNonce) MarshalJSON() ([]byte, error) {
	return json.Marshal(n.String())
}

func (n *EthNonce) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	s = strings.Replace(s, "0x", "", -1)
	if len(s)%2 == 1 {
		s = "0" + s
	}

	decoded, err := hex.DecodeString(s)
	if err != nil {
		return err
	}
	copy(n[:], decoded[:8])
	return nil
}

type EthAddress [EthAddressLength]byte

// EthAddressFromPubKey returns the Ethereum address corresponding to an
// uncompressed secp256k1 public key.
func EthAddressFromPubKey(pubk []byte) ([]byte, error) {
	// if we get an uncompressed public key (that's what we get from the library,
	// but putting this check here for defensiveness), strip the prefix
	const pubKeyLen = 65
	if len(pubk) != pubKeyLen {
		return nil, fmt.Errorf("public key should have %d in length, but got %d", pubKeyLen, len(pubk))
	}
	if pubk[0] != 0x04 {
		return nil, fmt.Errorf("expected first byte of secp256k1 to be 0x04 (uncompressed)")
	}
	pubk = pubk[1:]

	// Calculate the Ethereum address based on the keccak hash of the pubkey.
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(pubk)
	ethAddr := hasher.Sum(nil)[12:]
	return ethAddr, nil
}

var maskedIDPrefix = [20 - 8]byte{0xff}

func IsEthAddress(addr address.Address) bool {
	if addr.Protocol() != address.Delegated {
		return false
	}
	payload := addr.Payload()
	namespace, offset, err := varint.FromUvarint(payload)
	if err != nil {
		return false
	}

	payload = payload[offset:]

	return namespace == builtintypes.EthereumAddressManagerActorID && len(payload) == 20 && !bytes.HasPrefix(payload, maskedIDPrefix[:])
}

func EthAddressFromActorID(id abi.ActorID) EthAddress {
	var ethaddr EthAddress
	ethaddr[0] = 0xff
	binary.BigEndian.PutUint64(ethaddr[12:], uint64(id))
	return ethaddr
}

func EthAddressFromFilecoinAddress(addr address.Address) (EthAddress, error) {
	switch addr.Protocol() {
	case address.ID:
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return EthAddress{}, err
		}
		return EthAddressFromActorID(abi.ActorID(id)), nil
	case address.Delegated:
		payload := addr.Payload()
		namespace, n, err := varint.FromUvarint(payload)
		if err != nil {
			return EthAddress{}, fmt.Errorf("invalid delegated address namespace in: %s", addr)
		}
		payload = payload[n:]
		if namespace != builtintypes.EthereumAddressManagerActorID {
			return EthAddress{}, ErrInvalidAddress
		}
		ethAddr, err := CastEthAddress(payload)
		if err != nil {
			return EthAddress{}, err
		}
		if ethAddr.IsMaskedID() {
			return EthAddress{}, fmt.Errorf("f410f addresses cannot embed masked-ID payloads: %s", ethAddr)
		}
		return ethAddr, nil
	}
	return EthAddress{}, ErrInvalidAddress
}

// ParseEthAddress parses an Ethereum address from a hex string.
func ParseEthAddress(s string) (EthAddress, error) {
	b, err := decodeHexString(s, EthAddressLength)
	if err != nil {
		return EthAddress{}, err
	}
	var h EthAddress
	copy(h[EthAddressLength-len(b):], b)
	return h, nil
}

// CastEthAddress interprets bytes as an EthAddress, performing some basic checks.
func CastEthAddress(b []byte) (EthAddress, error) {
	var a EthAddress
	if len(b) != EthAddressLength {
		return EthAddress{}, fmt.Errorf("cannot parse bytes into an EthAddress: incorrect input length")
	}
	copy(a[:], b[:])
	return a, nil
}

func (ea EthAddress) String() string {
	return "0x" + hex.EncodeToString(ea[:])
}

func (ea EthAddress) MarshalJSON() ([]byte, error) {
	return json.Marshal(ea.String())
}

func (ea *EthAddress) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	addr, err := ParseEthAddress(s)
	if err != nil {
		return err
	}
	copy(ea[:], addr[:])
	return nil
}

func (ea EthAddress) IsMaskedID() bool {
	return bytes.HasPrefix(ea[:], maskedIDPrefix[:])
}

func (ea EthAddress) ToFilecoinAddress() (address.Address, error) {
	if ea.IsMaskedID() {
		// This is a masked ID address.
		id := binary.BigEndian.Uint64(ea[12:])
		return address.NewIDAddress(id)
	}

	// Otherwise, translate the address into an address controlled by the
	// Ethereum Address Manager.
	addr, err := address.NewDelegatedAddress(builtintypes.EthereumAddressManagerActorID, ea[:])
	if err != nil {
		return address.Undef, fmt.Errorf("failed to translate supplied address (%s) into a "+
			"Filecoin f4 address: %w", hex.EncodeToString(ea[:]), err)
	}
	return addr, nil
}

// This API assumes that if an ID address is passed in, it doesn't have an equivalent
// delegated address
func TryEthAddressFromFilecoinAddress(addr address.Address, allowID bool) (EthAddress, bool, error) {
	switch addr.Protocol() {
	case address.ID:
		if !allowID {
			return EthAddress{}, false, nil
		}
		id, err := address.IDFromAddress(addr)
		if err != nil {
			return EthAddress{}, false, err
		}
		var ethaddr EthAddress
		ethaddr[0] = 0xff
		binary.BigEndian.PutUint64(ethaddr[12:], id)
		return ethaddr, true, nil
	case address.Delegated:
		payload := addr.Payload()
		namespace, n, err := varint.FromUvarint(payload)
		if err != nil {
			return EthAddress{}, false, fmt.Errorf("invalid delegated address namespace in: %s", addr)
		}
		payload = payload[n:]
		if namespace == builtintypes.EthereumAddressManagerActorID {
			addr, err := CastEthAddress(payload)
			return addr, err == nil, err
		}
	}
	return EthAddress{}, false, nil
}

type EthHash [EthHashLength]byte

func (h EthHash) MarshalJSON() ([]byte, error) {
	return json.Marshal(h.String())
}

func (h *EthHash) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	hash, err := ParseEthHash(s)
	if err != nil {
		return err
	}
	copy(h[:], hash[:])
	return nil
}

func (h EthHash) String() string {
	return "0x" + hex.EncodeToString(h[:])
}

// Should ONLY be used for blocks and Filecoin messages. Eth transactions expect a different hashing scheme.
func (h EthHash) ToCid() cid.Cid {
	// err is always nil
	mh, _ := mh.EncodeName(h[:], "blake2b-256")

	return cid.NewCidV1(cid.DagCBOR, mh)
}

func decodeHexString(s string, expectedLen int) ([]byte, error) {
	s = handleHexStringPrefix(s)
	if len(s) != expectedLen*2 {
		return nil, fmt.Errorf("expected hex string length sans prefix %d, got %d", expectedLen*2, len(s))
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse hex value: %w", err)
	}
	return b, nil
}

func DecodeHexString(s string) ([]byte, error) {
	s = handleHexStringPrefix(s)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("cannot parse hex value: %w", err)
	}
	return b, nil
}

func DecodeHexStringTrimSpace(s string) ([]byte, error) {
	return DecodeHexString(strings.TrimSpace(s))
}

func handleHexStringPrefix(s string) string {
	// Strip the leading 0x or 0X prefix since hex.DecodeString does not support it.
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}
	// Sometimes clients will omit a leading zero in a byte; pad so we can decode correctly.
	if len(s)%2 == 1 {
		s = "0" + s
	}
	return s
}

func EthHashFromCid(c cid.Cid) (EthHash, error) {
	hash, found := bytes.CutPrefix(c.Bytes(), expectedHashPrefix)
	if !found {
		return EthHash{}, fmt.Errorf("CID does not have the expected prefix")
	}

	if len(hash) != EthHashLength {
		// this shouldn't be possible since the prefix has the length, but just in case
		return EthHash{}, fmt.Errorf("CID hash length is not 32 bytes")
	}

	var h EthHash
	copy(h[:], hash)
	return h, nil
}

func ParseEthHash(s string) (EthHash, error) {
	b, err := decodeHexString(s, EthHashLength)
	if err != nil {
		return EthHash{}, err
	}
	var h EthHash
	copy(h[EthHashLength-len(b):], b)
	return h, nil
}

func EthHashFromTxBytes(b []byte) EthHash {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(b)
	hash := hasher.Sum(nil)

	var ethHash EthHash
	copy(ethHash[:], hash)
	return ethHash
}

func EthBloomSet(f EthBytes, data []byte) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(data)
	hash := hasher.Sum(nil)

	for i := 0; i < 3; i++ {
		n := binary.BigEndian.Uint16(hash[i*2:]) % EthBloomSize
		f[(EthBloomSize/8)-(n/8)-1] |= 1 << (n % 8)
	}
}

type EthFeeHistory struct {
	OldestBlock   EthUint64      `json:"oldestBlock"`
	BaseFeePerGas []EthBigInt    `json:"baseFeePerGas"`
	GasUsedRatio  []float64      `json:"gasUsedRatio"`
	Reward        *[][]EthBigInt `json:"reward,omitempty"`
}

type EthTxReceipt struct {
	TransactionHash  EthHash     `json:"transactionHash"`
	TransactionIndex EthUint64   `json:"transactionIndex"`
	BlockHash        EthHash     `json:"blockHash"`
	BlockNumber      EthUint64   `json:"blockNumber"`
	From             EthAddress  `json:"from"`
	To               *EthAddress `json:"to"`
	// Logs
	// LogsBloom
	StateRoot         EthHash     `json:"root"`
	Status            EthUint64   `json:"status"`
	ContractAddress   *EthAddress `json:"contractAddress"`
	CumulativeGasUsed EthUint64   `json:"cumulativeGasUsed"`
	GasUsed           EthUint64   `json:"gasUsed"`
	EffectiveGasPrice EthBigInt   `json:"effectiveGasPrice"`
	LogsBloom         EthBytes    `json:"logsBloom"`
	Logs              []EthLog    `json:"logs"`
	Type              EthUint64   `json:"type"`
}

type EthFilterID EthHash

func (h EthFilterID) MarshalJSON() ([]byte, error) {
	return (EthHash)(h).MarshalJSON()
}

func (h *EthFilterID) UnmarshalJSON(b []byte) error {
	return (*EthHash)(h).UnmarshalJSON(b)
}

func (h EthFilterID) String() string {
	return (EthHash)(h).String()
}

// An opaque identifier generated by the Lotus node to refer to an active subscription.
type EthSubscriptionID EthHash

func (h EthSubscriptionID) MarshalJSON() ([]byte, error) {
	return (EthHash)(h).MarshalJSON()
}

func (h *EthSubscriptionID) UnmarshalJSON(b []byte) error {
	return (*EthHash)(h).UnmarshalJSON(b)
}

func (h EthSubscriptionID) String() string {
	return (EthHash)(h).String()
}

type EthFilterSpec struct {
	// Interpreted as an epoch (in hex) or one of "latest" for last mined block, "earliest" for first,
	// "pending" for not yet committed messages.
	// Optional, default: "latest".
	FromBlock *string `json:"fromBlock,omitempty"`

	// Interpreted as an epoch (in hex) or one of "latest" for last mined block, "earliest" for first,
	// "pending" for not yet committed messages.
	// Optional, default: "latest".
	ToBlock *string `json:"toBlock,omitempty"`

	// Actor address or a list of addresses from which event logs should originate.
	// Optional, default nil.
	// The JSON decoding must treat a string as equivalent to an array with one value, for example
	// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
	Address EthAddressList `json:"address"`

	// List of topics to be matched.
	// Optional, default: empty list
	Topics EthTopicSpec `json:"topics"`

	// Restricts event logs returned to those emitted from messages contained in this tipset.
	// If BlockHash is present in the filter criteria, then neither FromBlock nor ToBlock are allowed.
	// Added in EIP-234
	BlockHash *EthHash `json:"blockHash,omitempty"`
}

// EthAddressList represents a list of addresses.
// The JSON decoding must treat a string as equivalent to an array with one value, for example
// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
type EthAddressList []EthAddress

func (e *EthAddressList) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	if len(b) > 0 && b[0] == '[' {
		var addrs []EthAddress
		err := json.Unmarshal(b, &addrs)
		if err != nil {
			return err
		}
		*e = addrs
		return nil
	}
	var addr EthAddress
	err := json.Unmarshal(b, &addr)
	if err != nil {
		return err
	}
	*e = []EthAddress{addr}
	return nil
}

// TopicSpec represents a specification for matching by topic. An empty spec means all topics
// will be matched. Otherwise topics are matched conjunctively in the first dimension of the
// slice and disjunctively in the second dimension. Topics are matched in order.
// An event log with topics [A, B] will be matched by the following topic specs:
// [] "all"
// [[A]] "A in first position (and anything after)"
// [nil, [B] ] "anything in first position AND B in second position (and anything after)"
// [[A], [B]] "A in first position AND B in second position (and anything after)"
// [[A, B], [A, B]] "(A OR B) in first position AND (A OR B) in second position (and anything after)"
//
// The JSON decoding must treat string values as equivalent to arrays with one value, for example
// { "A", [ "B", "C" ] } must be decoded as [ [ A ], [ B, C ] ]
type EthTopicSpec []EthHashList

// EthHashList represents a list of EthHashes.
// The JSON decoding treats string values as equivalent to arrays with one value.
type EthHashList []EthHash

func (e *EthHashList) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	if len(b) > 0 && b[0] == '[' {
		var hashes []EthHash
		err := json.Unmarshal(b, &hashes)
		if err != nil {
			return err
		}
		*e = hashes
		return nil
	}
	var hash EthHash
	err := json.Unmarshal(b, &hash)
	if err != nil {
		return err
	}
	*e = []EthHash{hash}
	return nil
}

// FilterResult represents the response from executing a filter: a list of block hashes, a list of transaction hashes
// or a list of logs
// This is a union type. Only one field will be populated.
// The JSON encoding must produce an array of the populated field.
type EthFilterResult struct {
	Results []interface{}
}

func (h EthFilterResult) MarshalJSON() ([]byte, error) {
	if h.Results != nil {
		return json.Marshal(h.Results)
	}
	return []byte{'[', ']'}, nil
}

func (h *EthFilterResult) UnmarshalJSON(b []byte) error {
	if bytes.Equal(b, []byte{'n', 'u', 'l', 'l'}) {
		return nil
	}
	err := json.Unmarshal(b, &h.Results)
	return err
}

// EthLog represents the results of an event filter execution.
type EthLog struct {
	// Address is the address of the actor that produced the event log.
	Address EthAddress `json:"address"`

	// Data is the value of the event log, excluding topics
	Data EthBytes `json:"data"`

	// List of topics associated with the event log.
	Topics []EthHash `json:"topics"`

	// Following fields are derived from the transaction containing the log

	// Indicates whether the log was removed due to a chain reorganization.
	Removed bool `json:"removed"`

	// LogIndex is the index of the event log in the sequence of events produced by the message execution.
	// (this is the index in the events AMT on the message receipt)
	LogIndex EthUint64 `json:"logIndex"`

	// TransactionIndex is the index in the tipset of the transaction that produced the event log.
	// The index corresponds to the sequence of messages produced by ChainGetParentMessages
	TransactionIndex EthUint64 `json:"transactionIndex"`

	// TransactionHash is the hash of the RLP message that produced the event log.
	TransactionHash EthHash `json:"transactionHash"`

	// BlockHash is the hash of the tipset containing the message that produced the log.
	BlockHash EthHash `json:"blockHash"`

	// BlockNumber is the epoch of the tipset containing the message.
	BlockNumber EthUint64 `json:"blockNumber"`
}

// EthSubscribeParams handles raw jsonrpc params for eth_subscribe
type EthSubscribeParams struct {
	EventType string
	Params    *EthSubscriptionParams
}

func (e *EthSubscribeParams) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	err := json.Unmarshal(b, &params)
	if err != nil {
		return err
	}
	switch len(params) {
	case 2:
		err = json.Unmarshal(params[1], &e.Params)
		if err != nil {
			return err
		}
		fallthrough
	case 1:
		err = json.Unmarshal(params[0], &e.EventType)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("expected 1 or 2 params, got %d", len(params))
	}
	return nil
}

func (e EthSubscribeParams) MarshalJSON() ([]byte, error) {
	if e.Params != nil {
		return json.Marshal([]interface{}{e.EventType, e.Params})
	}
	return json.Marshal([]interface{}{e.EventType})
}

type EthSubscriptionParams struct {
	// List of topics to be matched.
	// Optional, default: empty list
	Topics EthTopicSpec `json:"topics,omitempty"`

	// Actor address or a list of addresses from which event logs should originate.
	// Optional, default nil.
	// The JSON decoding must treat a string as equivalent to an array with one value, for example
	// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
	Address EthAddressList `json:"address"`
}

type EthSubscriptionResponse struct {
	// The persistent identifier for the subscription which can be used to unsubscribe.
	SubscriptionID EthSubscriptionID `json:"subscription"`

	// The object matching the subscription. This may be a Block (tipset), a Transaction (message) or an EthLog
	Result interface{} `json:"result"`
}

func GetContractEthAddressFromCode(sender EthAddress, salt [32]byte, initcode []byte) (EthAddress, error) {
	hasher := sha3.NewLegacyKeccak256()
	hasher.Write(initcode)
	inithash := hasher.Sum(nil)

	hasher.Reset()
	hasher.Write([]byte{0xff})
	hasher.Write(sender[:])
	hasher.Write(salt[:])
	hasher.Write(inithash)

	ethAddr, err := CastEthAddress(hasher.Sum(nil)[12:])
	if err != nil {
		return [20]byte{}, err
	}

	return ethAddr, nil
}

// EthEstimateGasParams handles raw jsonrpc params for eth_estimateGas
type EthEstimateGasParams struct {
	Tx       EthCall
	BlkParam *EthBlockNumberOrHash
}

func (e *EthEstimateGasParams) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	err := json.Unmarshal(b, &params)
	if err != nil {
		return err
	}

	switch len(params) {
	case 2:
		err = json.Unmarshal(params[1], &e.BlkParam)
		if err != nil {
			return err
		}
		fallthrough
	case 1:
		err = json.Unmarshal(params[0], &e.Tx)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("expected 1 or 2 params, got %d", len(params))
	}

	return nil
}

func (e EthEstimateGasParams) MarshalJSON() ([]byte, error) {
	if e.BlkParam != nil {
		return json.Marshal([]interface{}{e.Tx, e.BlkParam})
	}
	return json.Marshal([]interface{}{e.Tx})
}

// EthFeeHistoryParams handles raw jsonrpc params for eth_feeHistory
type EthFeeHistoryParams struct {
	BlkCount          EthUint64
	NewestBlkNum      string
	RewardPercentiles *[]float64
}

func (e *EthFeeHistoryParams) UnmarshalJSON(b []byte) error {
	var params []json.RawMessage
	err := json.Unmarshal(b, &params)
	if err != nil {
		return err
	}
	switch len(params) {
	case 3:
		err = json.Unmarshal(params[2], &e.RewardPercentiles)
		if err != nil {
			return err
		}
		fallthrough
	case 2:
		err = json.Unmarshal(params[1], &e.NewestBlkNum)
		if err != nil {
			return err
		}
		err = json.Unmarshal(params[0], &e.BlkCount)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("expected 2 or 3 params, got %d", len(params))
	}
	return nil
}

func (e EthFeeHistoryParams) MarshalJSON() ([]byte, error) {
	if e.RewardPercentiles != nil {
		return json.Marshal([]interface{}{e.BlkCount, e.NewestBlkNum, e.RewardPercentiles})
	}
	return json.Marshal([]interface{}{e.BlkCount, e.NewestBlkNum})
}

type EthBlockNumberOrHash struct {
	// PredefinedBlock can be one of "earliest", "pending" or "latest". We could merge this
	// field with BlockNumber if the latter could store negative numbers representing
	// each predefined value (e.g. -1 for "earliest", -2 for "pending" and -3 for "latest")
	PredefinedBlock *string `json:"-"`

	BlockNumber      *EthUint64 `json:"blockNumber,omitempty"`
	BlockHash        *EthHash   `json:"blockHash,omitempty"`
	RequireCanonical bool       `json:"requireCanonical,omitempty"`
}

func NewEthBlockNumberOrHashFromPredefined(predefined string) EthBlockNumberOrHash {
	return EthBlockNumberOrHash{
		PredefinedBlock:  &predefined,
		BlockNumber:      nil,
		BlockHash:        nil,
		RequireCanonical: false,
	}
}

func NewEthBlockNumberOrHashFromNumber(number EthUint64) EthBlockNumberOrHash {
	return EthBlockNumberOrHash{
		PredefinedBlock:  nil,
		BlockNumber:      &number,
		BlockHash:        nil,
		RequireCanonical: false,
	}
}

func NewEthBlockNumberOrHashFromHexString(str string) (EthBlockNumberOrHash, error) {
	// check if block param is a number (decimal or hex)
	var num EthUint64
	err := num.UnmarshalJSON([]byte(str))
	if err != nil {
		return NewEthBlockNumberOrHashFromNumber(0), err
	}

	return EthBlockNumberOrHash{
		PredefinedBlock:  nil,
		BlockNumber:      &num,
		BlockHash:        nil,
		RequireCanonical: false,
	}, nil
}

func (e EthBlockNumberOrHash) MarshalJSON() ([]byte, error) {
	if e.PredefinedBlock != nil {
		return json.Marshal(*e.PredefinedBlock)
	}

	type tmpStruct EthBlockNumberOrHash
	return json.Marshal(tmpStruct(e))
}

func (e *EthBlockNumberOrHash) UnmarshalJSON(b []byte) error {
	// we first try to unmarshal into a EthBlockNumberOrHash struct to check
	// if the block param is a block hash or block number (see EIP-1898). We use
	// a temporary struct to avoid infinite recursion.
	type tmpStruct EthBlockNumberOrHash
	var tmp tmpStruct
	if err := json.Unmarshal(b, &tmp); err == nil {
		if tmp.BlockNumber != nil && tmp.BlockHash != nil {
			return errors.New("cannot specify both blockNumber and blockHash")
		}

		*e = EthBlockNumberOrHash(tmp)
		return nil
	}

	// check if block param is once of the special strings
	var str string
	err := json.Unmarshal(b, &str)
	if err != nil {
		return err
	}
	if str == "earliest" || str == "pending" || str == "latest" {
		e.PredefinedBlock = &str
		return nil
	}

	// check if block param is a number (decimal or hex)
	var num EthUint64
	if err := num.UnmarshalJSON(b); err == nil {
		e.BlockNumber = &num
		return nil
	}

	// check if input is a block hash (66 characters long)
	if len(str) == 66 && strings.HasPrefix(str, "0x") {
		hash, err := ParseEthHash(str)
		if err != nil {
			return err
		}
		e.BlockHash = &hash
		return nil
	}

	return errors.New("invalid block param")
}

type EthTrace struct {
	Type         string `json:"type"`
	Error        string `json:"error,omitempty"`
	Subtraces    int    `json:"subtraces"`
	TraceAddress []int  `json:"traceAddress"`
	Action       any    `json:"action"`
	Result       any    `json:"result"`
}

type EthTraceBlock struct {
	*EthTrace
	BlockHash           EthHash `json:"blockHash"`
	BlockNumber         int64   `json:"blockNumber"`
	TransactionHash     EthHash `json:"transactionHash"`
	TransactionPosition int     `json:"transactionPosition"`
}

type EthTraceReplayBlockTransaction struct {
	Output          EthBytes    `json:"output"`
	StateDiff       *string     `json:"stateDiff"`
	Trace           []*EthTrace `json:"trace"`
	TransactionHash EthHash     `json:"transactionHash"`
	VMTrace         *string     `json:"vmTrace"`
}

type EthTraceTransaction struct {
	*EthTrace
	BlockHash           EthHash `json:"blockHash"`
	BlockNumber         int64   `json:"blockNumber"`
	TransactionHash     EthHash `json:"transactionHash"`
	TransactionPosition int     `json:"transactionPosition"`
}

type EthCallTraceAction struct {
	CallType string     `json:"callType"`
	From     EthAddress `json:"from"`
	To       EthAddress `json:"to"`
	Gas      EthUint64  `json:"gas"`
	Value    EthBigInt  `json:"value"`
	Input    EthBytes   `json:"input"`
}

type EthCallTraceResult struct {
	GasUsed EthUint64 `json:"gasUsed"`
	Output  EthBytes  `json:"output"`
}

type EthCreateTraceAction struct {
	From  EthAddress `json:"from"`
	Gas   EthUint64  `json:"gas"`
	Value EthBigInt  `json:"value"`
	Init  EthBytes   `json:"init"`
}

type EthCreateTraceResult struct {
	Address *EthAddress `json:"address,omitempty"`
	GasUsed EthUint64   `json:"gasUsed"`
	Code    EthBytes    `json:"code"`
}

type EthTraceFilterResult struct {
	*EthTrace
	BlockHash           EthHash `json:"blockHash"`
	BlockNumber         int64   `json:"blockNumber"`
	TransactionHash     EthHash `json:"transactionHash"`
	TransactionPosition int     `json:"transactionPosition"`
}

// EthTraceFilterCriteria defines the criteria for filtering traces.
type EthTraceFilterCriteria struct {
	// Interpreted as an epoch (in hex) or one of "latest" for last mined block, "pending" for not yet committed messages.
	// Optional, default: "latest".
	// Note: "earliest" is not a permitted value.
	FromBlock *string `json:"fromBlock,omitempty"`

	// Interpreted as an epoch (in hex) or one of "latest" for last mined block, "pending" for not yet committed messages.
	// Optional, default: "latest".
	// Note: "earliest" is not a permitted value.
	ToBlock *string `json:"toBlock,omitempty"`

	// Actor address or a list of addresses from which transactions that generate traces should originate.
	// Optional, default: nil.
	// The JSON decoding must treat a string as equivalent to an array with one value, for example
	// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
	FromAddress EthAddressList `json:"fromAddress,omitempty"`

	// Actor address or a list of addresses to which transactions that generate traces are sent.
	// Optional, default: nil.
	// The JSON decoding must treat a string as equivalent to an array with one value, for example
	// "0x8888f1f195afa192cfee86069858" must be decoded as [ "0x8888f1f195afa192cfee86069858" ]
	ToAddress EthAddressList `json:"toAddress,omitempty"`

	// After specifies the offset for pagination of trace results. The number of traces to skip before returning results.
	// Optional, default: nil.
	After *EthUint64 `json:"after,omitempty"`

	// Limits the number of traces returned.
	// Optional, default: all traces.
	Count *EthUint64 `json:"count,omitempty"`
}
