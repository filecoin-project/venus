package main

import (
	"fmt"
	"os"
	"path/filepath"

	ffi "github.com/filecoin-project/filecoin-ffi"

	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/block"
	"github.com/filecoin-project/go-filecoin/internal/pkg/discovery"
	"github.com/filecoin-project/go-filecoin/internal/pkg/encoding/gen"
	"github.com/filecoin-project/go-filecoin/internal/pkg/protocol/retrieval"
	"github.com/filecoin-project/go-filecoin/internal/pkg/types"
	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/actor"
)

// var base = "/tmp/encoding_gen"
var base = "."

func main() {
	logging.SetAllLoggers(logging.LevelDebug)

	if err := gen.WriteToFile(filepath.Join(base, "actor/actor_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "actor",
		actor.Actor{}, // actor/actor.go
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// cbor.BigIntAtlasEntry,          // actor/built-in/miner.go XXX: atlas

	// struct{}{},                     // actor/built-in/storagemarket.go XXX: unit aint working
	// address.Address{}, // address/address.go XXX: custom

	if err := gen.WriteToFile(filepath.Join(base, "block/block_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "block",
		block.Block{},  // block/block.go
		block.Ticket{}, // block/ticket.go
		// block.TipSetKey{}, // block/tipset_key.go XXX: custom
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := gen.WriteToFile(filepath.Join(base, "proofs/sectorbuilder/sectorbuilder_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "sectorbuilder",
		ffi.PublicPieceInfo{},
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := gen.WriteToFile(filepath.Join(base, "discovery/discovery_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "discovery",
		discovery.HelloMessage{}, // discovery/hello_protocol.go
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if err := gen.WriteToFile(filepath.Join(base, "protocol/retrieval/retrieval_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "retrieval",
		retrieval.RetrievePieceRequest{},  // protocol/retrieval/types.go
		retrieval.RetrievePieceResponse{}, // protocol/retrieval/types.go
		retrieval.RetrievePieceChunk{},    // protocol/retrieval/types.go
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	// storage.dealsAwaitingSeal{}, // protocol/storage/deals_awaiting_seal.go XXX: private struct

	if err := gen.WriteToFile(filepath.Join(base, "types/types_encoding_gen.go"), gen.IpldCborTypeEncodingGenerator{}, "types",
		// types.AttoFIL{}, // types/atto_file.go XXX: custom
		// types.BlockHeight{}, // types/block_height.go XXX: custom
		// types.BytesAmount{}, // types/bytes_amount.go XXX: custom
		// types.ChannelID{}, // types/channel_id.go XXX: custom
		types.Commitments{}, // types/commitments.go
		types.FaultSet{},    // types/fault_set.go
		// types.IntSet{},      // types/intset.go XXX: custom
		types.KeyInfo{}, // types/keyinfo.go
		// types.MessageCollection{}, // types/message.go // XXX: array
		// types.ReceiptCollection{}, // types/message.go // XXX: array
		types.UnsignedMessage{}, // types/message.go
		types.TxMeta{},          // types/message.go
		types.SignedMessage{},   // types/signed_message.go
		// types.SignedMessageCollection{}, // types/signed_message.go // XXX: array
		// types.Uint64(0),   // types/uint64.go XXX: CUSTOM
	); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
