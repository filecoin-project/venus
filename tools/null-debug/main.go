package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/big"
	"net/http"
	"os"
	"strconv"
	
	"github.com/filecoin-project/go-filecoin/types"
)

// cMemo helps memoize c.  keys are "n, k, x"
var cMemo map[string]*big.Int

// pMemo helps memoize probabilities.  Keys are "p^n" or "(1-p)^n".
var pMemo map[string]*big.Float

func init() {
	cMemo = make(map[string]*big.Int)
	pMemo = make(map[string]*big.Float)
}

func main() {
	port := flag.Int("port", 3453, "port for reading filecoin chain data")
//	file := flag.String("file", "", "json file for reading out chain data")

	flag.Parse()
/*	if *file == "" {
		fmt.Fprintf(os.Stderr, "No file specified")
		os.Exit(1)
	}
	chain, err := readFile(*file)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read api")
		os.Exit(1)
	}
*/
	
	chain, err := readAPI(*port)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not read api")
		os.Exit(1)
	}
	
//	fmt.Printf("chain: %v\n", chain)
	fmt.Printf("chain len: %d\n", len(chain))

	counts := countNullBlks(chain)
	fmt.Printf("counts: %v\n", counts)
	maxCount := maxKey(counts)
	p := probNoMoreThanX(maxCount - 1, len(chain))
	fmt.Printf("probability of seeing fewer than %d null blocks: %v\n", maxCount, p)
	q := probNoMoreThanX(maxCount, len(chain))
	fmt.Printf("probability of seeing %d or fewer null blocks: %v\n", maxCount, q)
}

func readFile(filename string) ([][]types.Block, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	return readJSONStream(file)
}

func readAPI(port int) ([][]types.Block, error) {
	url := fmt.Sprintf("http://localhost:"+strconv.Itoa(port)+"/api/chain/ls?encoding=json&long=true&stream-channels=true")
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", "go-ipfs-cmds/http")
	req.Header.Add("Connection", "close")
	req.Header.Add("Content-Type", "application/octet-stream")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	return readJSONStream(resp.Body)
}

func readJSONStream(r io.Reader) ([][]types.Block, error) {
	var chain [][]types.Block
	dec := json.NewDecoder(r)
	for dec.More() {
		var ts []types.Block		
		err := dec.Decode(&ts)
		if err != nil {
			return nil, err
		}
		chain = append(chain, ts)
	}

	return chain, nil
}


// Return a frequency map from null block counts to occurrence number
func countNullBlks(chain [][]types.Block) map[int]int {
	freq := make(map[int]int)
//	fmt.Printf("chain[1:]: %v\n", chain[1:])
	for i, blks := range chain[1:] {
		// be default chain is parsed in reverse order so we subtract backwards
		count := int(uint64(chain[i][0].Height) - uint64(blks[0].Height)) - 1
		if _, ok := freq[count]; !ok {
			freq[count] = 0
		}
		freq[count] += 1
	}
	return freq
}

func maxKey(m map[int]int) int {
	maxKey := -1
	for k, _ := range m {
		if k > maxKey {
			maxKey = k
		}
	}
	return maxKey
}

func memoKey(n, k, x int) string {
	return fmt.Sprintf("%d, %d, %d", n, k, x)
}

// c(n, k, x) is the number of chains of length n in which exactly k null
// blocks occur, but no more than x of these occur consecutively.
func c(n, k, x int) *big.Int {
	if val, ok := cMemo[memoKey(n,k,x)]; ok {
		return val
	}
	
	if k <= x {
		z := big.NewInt(int64(0))
		ret := z.Binomial(int64(n), int64(k))
		return ret
	}
	if k == n && k > x {
		return big.NewInt(int64(0))
	}
	ret := big.NewInt(int64(0))
	for j := 0; j <= x; j++ {
		ret.Add(ret, c(n-1-j, k-j, x))
	}
	cMemo[memoKey(n,k,x)] = ret
	return ret
}

// Return the key used for get/putting memoized (1-p)^n or p^n values
func pMemoKey(nullBlock bool, exponent int) string {
	if nullBlock {
		return fmt.Sprintf("p^%d", exponent)
	}
	return fmt.Sprintf("(1-p)^%d", exponent)
}

// An implementation of exponentiating big.Floats that memoizes previous values
// to speed things up.  When nullBlock == true it calculates p^exponent where p is the
// probability of seeinga nullBlock.  When nullBlokc == false it calculates
// (1-p)^exponent.
func calcProb(nullBlock bool, exponent int) *big.Float {
	if val, ok := pMemo[pMemoKey(nullBlock, exponent)]; ok {
		return val
	}

	if exponent == 0 {
		return big.NewFloat(1.0)
	}

	if exponent < 0 {
		panic("ah come on")
	}

	var base big.Float
	if nullBlock {
		base.SetFloat64(probNullBlk())
	} else {
		base.SetFloat64(1.0 - probNullBlk())
	}

	if exponent == 1 {
		return &base
	}
	
	prob := big.NewFloat(1.0)
	prevVal := calcProb(nullBlock, exponent - 1)
	prob.Mul(prevVal, &base)

	pMemo[pMemoKey(nullBlock, exponent)] = prob
	return prob
}

// Return the probability of the longest run of null blocks having length
// no more than x in a chain of n blocks.  Note that the analysis only works
// for a  biased coin of constant probability.  It will be useful for future
// tests to extend the analysis to a biased coin that varies in probability at
// each round.
func probNoMoreThanX(x int, n int) float64 {
	prob := big.NewFloat(0.0)
	for k := 0; k <= n; k++ {
		var numChains big.Float
		numChains.SetInt(c(n, k, x))
//		fmt.Printf("k=%d, numChains=%v\n", k, numChains)
		var probChain big.Float
		probChain.Mul(calcProb(true, k), calcProb(false, n-k))
//		fmt.Printf("k=%d, probChain=%v\n", k, probChain)
		probAny := big.NewFloat(0.0)
		probAny.Mul(&probChain, &numChains)
		prob.Add(prob, probAny)
	}
	p, acc := prob.Float64()
	if acc != big.Exact {
		fmt.Printf("WARNING: accuracy is not exact")
	}
	return p
}

// Return the probability of seeing a null block.  Assumes that
// there is a fixed distribution of 5 miners each with 1/5th of the total
// power.
func probNullBlk() float64 {
	return math.Pow(4.0/5.0, 5.0)
}


	
