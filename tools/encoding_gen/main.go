package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/encoding/gen"
	"github.com/filecoin-project/go-filecoin/types"
)

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
}
