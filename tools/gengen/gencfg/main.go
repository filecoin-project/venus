package main

import (
	"encoding/json"
	"fmt"
	"os"

	commcid "github.com/filecoin-project/go-fil-commcid"
)

func main() {
	if len(os.Args) != 2 {
		panic("incorrect number of arguments")
	}
	byteSliceJSON := os.Args[1]

	var bytes []byte
	err := json.Unmarshal([]byte(byteSliceJSON), &bytes)
	if err != nil {
		panic(err)
	}

	cid, _ := commcid.DataCommitmentV1ToCID(bytes)
	fmt.Printf("%s\n", cid)
}
