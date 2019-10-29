package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/ipfs/go-cid"
	logging "github.com/ipfs/go-log"

	"github.com/filecoin-project/go-filecoin/internal/pkg/vm/address"
	"github.com/filecoin-project/go-filecoin/tools/faucet/limiter"
)

var log = logging.Logger("faucet")

// Tick interval to cleanup wallet addrs that have passed the expiry time
var limiterCleanTick = time.Minute * 15

// Default timeout between wallet fund requests
var defaultLimiterExpiry = time.Hour * 1

func init() {
	// Info level
	logging.SetAllLoggers(4)
}

type timeImpl struct{}

// Until returns the time.Duration until time.Time t
func (mt *timeImpl) Until(t time.Time) time.Duration {
	return time.Until(t)
}

func main() {
	filapi := flag.String("fil-api", "localhost:3453", "set the api address of the filecoin node to use")
	filwal := flag.String("fil-wallet", "", "(required) set the wallet address for the controlled filecoin node to send funds from")
	expiry := flag.Duration("limiter-expiry", defaultLimiterExpiry, "minimum time duration between faucet request to the same wallet addr")
	faucetval := flag.Int64("faucet-val", 500, "set the amount of fil to pay to each requester")
	flag.Parse()

	if *filwal == "" {
		fmt.Println("ERROR: must provide wallet address to send funds from")
		flag.Usage()
		return
	}

	addrLimiter := limiter.NewLimiter(&timeImpl{})

	// Clean the limiter every limiterCleanTick
	go func() {
		c := time.Tick(limiterCleanTick)
		for range c {
			addrLimiter.Clean()
		}
	}()

	http.HandleFunc("/", displayForm)
	http.HandleFunc("/tap", func(w http.ResponseWriter, r *http.Request) {
		target := r.FormValue("target")
		if target == "" {
			http.Error(w, "must specify a target address to send FIL to", 400)
			return
		}
		log.Infof("Request to send funds to: %s", target)

		addr, err := address.NewFromString(target)
		if err != nil {
			log.Errorf("failed to parse target address: %s %s", target, err)
			http.Error(w, fmt.Sprintf("Failed to parse target address %s %s", target, err.Error()), 400)
			return
		}

		if readyIn, ok := addrLimiter.Ready(target); !ok {
			log.Errorf("limit hit for target address %s", target)
			w.Header().Add("Retry-After", fmt.Sprintf("%d", int64(readyIn/time.Second)))
			http.Error(w, fmt.Sprintf("Too Many Requests, please wait %s", readyIn), http.StatusTooManyRequests)
			return
		}

		reqStr := fmt.Sprintf("http://%s/api/message/send?arg=%s&value=%d&from=%s&gas-price=1&gas-limit=0", *filapi, addr.String(), *faucetval, *filwal)
		log.Infof("Request URL: %s", reqStr)

		resp, err := http.Post(reqStr, "application/json", nil)
		if err != nil {
			log.Errorf("failed to Post request. Status: %s Error: %s", resp.Status, err)
			http.Error(w, err.Error(), 500)
			return
		}

		out, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			log.Errorf("failed to read response body: %s", err)
			http.Error(w, "failed to read response", 500)
			return
		}
		if resp.StatusCode != 200 {
			log.Errorf("status: %s body: %s", resp.Status, string(out))
			http.Error(w, "failed to send funds", 500)
			return
		}

		msgResp := struct{ Cid cid.Cid }{}

		// result should be a message cid
		if err := json.Unmarshal(out, &msgResp); err != nil {
			log.Errorf("json unmarshal from response failed: %s", err)
			log.Errorf("response data was: %s", out)
			http.Error(w, "faucet unmarshal failed", 500)
			return
		}
		msgcid := msgResp.Cid

		addrLimiter.Add(target, time.Now().Add(*expiry))

		log.Info("Request successful. Message CID: %s", msgcid.String())
		w.Header().Add("Message-Cid", msgcid.String())
		w.WriteHeader(200)
		fmt.Fprint(w, "Success! Message CID: ") // nolint: errcheck
		fmt.Fprintln(w, msgcid.String())        // nolint: errcheck
	})

	panic(http.ListenAndServe(":9797", nil))
}

const form = `
<html>
	<body>
		<h1> What is your wallet address </h1>
		<p> You can find this by running: </p>
		<tt> go-filecoin address ls </tt>
		<p> Address: </p>
		<form action="/tap" method="post">
			<input type="text" name="target" size="30" />
			<input type="submit" value="Submit" size="30" />
		</form>
	</body>
</html>
`

func displayForm(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, form) // nolint: errcheck
}
