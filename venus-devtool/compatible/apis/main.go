package main

import (
	"fmt"

	"github.com/filecoin-project/lotus/api"
)

func main() {
	fmt.Printf("%T\n", api.FullNode(nil))
}
