package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/encoding/gen"
	"github.com/filecoin-project/go-filecoin/types"
	whygen "github.com/whyrusleeping/cbor-gen"
)

type Point struct {
	X uint64
	Y uint64
}

func main() {
	err := gen.WriteToFile("/tmp/types_gen.go", gen.IpldCborTypeEncodingGenerator{}, "types",
		types.Ticket{},
		types.Message{},
		types.SignedMessage{},
		types.MessageReceipt{},
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = whygen.WriteTupleEncodersToFile("/tmp/types_whygen.go", "types",
		types.Ticket{},
		Point{},
		// types.Message{}, AttoFil needs to be part of it too
		// types.SignedMessage{},
		// types.MessageReceipt{}, XXX: it has a uint8 that is not supproted by whygen
	)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
