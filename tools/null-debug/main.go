package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	
	"github.com/filecoin-project/go-filecoin/types"
)

func main() {
	port := flag.Int("port", 3453, "port for reading filecoin chain data")

	flag.Parse()
	chain, err := readLS(*port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("chain: %v\n", chain)
	fmt.Printf("chain len: %d\n", len(chain))
}

func readLS(port int) ([]types.Block, error) {
	var chain []types.Block
	resp, err := http.Get("http://localhost:"+strconv.Itoa(port)+"/api/chain/ls?encoding=json&long=true")
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(&chain)
	if err != nil {
		return nil, err
	}	
	return chain, err
}
