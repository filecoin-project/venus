package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strconv"
)

type Miner struct {
	Owner string
	Power uint64
}

type SetupTemplate struct {
	Keys     []string          `json:"keys"`
	PreAlloc map[string]string `json:"preAlloc"`
	Miner    []Miner           `json:"miners"`
}

// this generates a setup.json file that can be passed to gengen
func main() {
	// the number of entries to generate in the output setup.json file
	numbPtr := flag.Int("count", 10, "how many entries to generate")
	flag.Parse()

	num := int(*numbPtr)

	st := new(SetupTemplate)
	st.Keys = make([]string, num)
	st.PreAlloc = make(map[string]string, num)

	for i := 0; i < num; i++ {
		st.Keys[i] = strconv.Itoa(i)
		st.PreAlloc[strconv.Itoa(i)] = "10000"
		m := Miner{
			Owner: strconv.Itoa(i),
			Power: 1000,
		}
		st.Miner = append(st.Miner, m)
	}

	setup, err := json.Marshal(st)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(setup))
}
