package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	port := flag.Int("port", 0, "port over which to serve genesis file via HTTP")
	genesisFilePath := flag.String(
		"genesis-file-path",
		filepath.Join(os.Getenv("GOPATH"), "src/github.com/filecoin-project/go-filecoin/fixtures/genesis.car"),
		"port over which to serve genesis file via HTTP\n",
	)
	flag.Parse()

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
