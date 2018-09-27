package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
)

type miner struct {
	Owner string
	Power uint64
}

type setupTemplate struct {
	Keys     []string          `json:"keys"`
	PreAlloc map[string]string `json:"preAlloc"`
	Miner    []miner           `json:"miners"`
}

// this generates a setup.json file that can be passed to gengen
func main() {
	// the number of entries to generate in the output setup.json file
	numbPtr := flag.Int("count", 10, "how many entries to generate")
	flag.Parse()

	num := *numbPtr

	st := new(setupTemplate)
	st.Keys = make([]string, num)
	st.PreAlloc = make(map[string]string, num)

	for i := 0; i < num; i++ {
		st.Keys[i] = strconv.Itoa(i)
		st.PreAlloc[strconv.Itoa(i)] = "10000"
		m := miner{
			Owner: strconv.Itoa(i),
			Power: 100,
		}
		st.Miner = append(st.Miner, m)
	}

	setup, err := json.Marshal(st)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(setup))
}
