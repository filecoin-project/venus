package main

import (
	"fmt"
	"os"

	"github.com/filecoin-project/go-filecoin/internal/pkg/constants"
)

func main() {
	if len(os.Args) != 2 {
		panic("incorrect number of arguments")
	}
	fakeCidString := os.Args[1]

	cidStr, err := constants.DefaultCidBuilder.Sum([]byte(fakeCidString))
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s\n", cidStr)
}
