package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
)

func main() {
	port := flag.Int("port", 0, "port over which to serve genesis file via HTTP")
	genesisFilePath := flag.String("genesis-file-path", "", "full path to the genesis file to be served")
	flag.Parse()

	if len(*genesisFilePath) == 0 {
		fmt.Println("Please specify a genesis to serve")
		os.Exit(1)
	}

	ServeGenesisFileAtPort(*genesisFilePath, *port)
}

// ServeGenesisFileAtPort will serve the genesis.car file via HTTP at the given port for download
func ServeGenesisFileAtPort(genesisFilePath string, port int) {
	http.HandleFunc("/genesis.car", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		http.ServeFile(w, r, genesisFilePath)
	})
	panic(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}
