package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/filecoin-project/go-filecoin/address"
	"gx/ipfs/QmZFbDTY9jfSBms2MchvYM9oYRbAF19K7Pby47yDBfpPrb/go-cid"
)

func main() {
	filapi := flag.String("fil-api", "localhost:3453", "set the api address of the filecoin node to use")
	faucetval := flag.Int64("faucet-val", 500, "set the amount of fil to pay to each requester")
	flag.Parse()

	http.HandleFunc("/tap", func(w http.ResponseWriter, r *http.Request) {
		target := r.FormValue("target")
		if target == "" {
			http.Error(w, "must specify a target address to send FIL to", 400)
			return
		}

		addr, err := address.NewFromString(target)
		if err != nil {
			http.Error(w, err.Error(), 400)
			return
		}

		resp, err := http.Post(fmt.Sprintf("http://%s/api/message/send?arg=%s&value=%d", *filapi, addr.String(), *faucetval), "application/json", nil)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		out, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("failed to read body: ", err)
			return
		}
		if resp.StatusCode != 200 {
			fmt.Println("Failed request: ", string(out))
			http.Error(w, "failed to send funds", 500)
			return
		}

		// result should be a message cid
		var msgcid cid.Cid
		if err := json.Unmarshal(out, &msgcid); err != nil {
			fmt.Println("json unmarshal from command failed: ", err)
			fmt.Println("data was: ", string(out))
			http.Error(w, "faucet unmarshal failed", 500)
			return
		}

		w.WriteHeader(200)
		fmt.Fprintln(w, msgcid.String())
	})

	panic(http.ListenAndServe(":9797", nil))
}
