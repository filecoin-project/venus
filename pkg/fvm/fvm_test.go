package fvm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/filecoin-project/go-state-types/builtin/v9/miner"
	"github.com/filecoin-project/venus/venus-shared/actors"
	"github.com/filecoin-project/venus/venus-shared/testutil"
	types "github.com/filecoin-project/venus/venus-shared/types"
	"github.com/stretchr/testify/assert"
)

func TestAA(t *testing.T) {
	a := []byte(`kEQAwKsJgVhgisjG9qYrruagH5TZz7pXjNzVGKzKJQmnSqjqXes2EjA9Gdy1SUooTlEVp5Pkdta/CZAyaOUDBkj6hbKS7TVzAIYYCaNfavPgOtVOtuirlJv4NCvAo+sTzB90LzJn4OEZggFYYLB7sP1mIEIQWcjvrOsrnFCF6gXrgnrxdDRYS824XOOo+b5u+nVTY8S2KnSwU/TYNAV5AszygE7Bd0Q9xbs3aWscllWUFWJrzNAh4DfAser8Zb4kXHPYplGAPCoA8xE5v4GCGgAlRtRYYLJXbMYq++5cyPqlUJfFC2T+1Narq3FisYkqDSnMqHxYuDgF2ASHZxZcl9cV63W1wBRIT2zdIWQ36m/kI538ILwo68+V0JfC85Xk2K++e5SMhCH5t4R3gy8N/xIsEkdlzoGCBFjAoyTtZ52IjMTRj5p9iV6fqR5es/DJOpOYl9ItPrBgYOg4NlV5UY7a/bZhKgrPFsWVrngRaLeyie9v3B/AmwGvmz+TbgPrDMgmtUhY/CqsCYT4vZ0gyLuWlgZ6IvWp5/rlCnRm6Il9aiC2JJtWz1EmOvp7wfuT/px2nj2MZd29v537ps+rHbEzdT8PmmPNcDVatDJlNEjQhJfgxA+JdjWJlbKz94a+0bILb2YUDUnFzpR6xGqEBw7Zt2wXc9OCA7eehtgqWCcAAXGg5AIgW1VUooZxeX8k65jX3NOepjfZdBnMD4Jn0SzXNV8fs0bYKlgnAAFxoOQCIKKLEdxJbo8yJSVv07DR3wjt0SwyLtSq1W7aeYVSRUIh2CpYJwABcaDkAiBrBep4UPyfRUMVzoFmiiAUsqsY7c05tMZ6Niyo42Se89gqWCcAAXGg5AIgEw89ZxAFSI9tea9WkZzKPJ/f+C2OGTNhjp0XEKdJxvvYKlgnAAFxoOQCIIXfzxYBCQK50Si/1pHj/rsiNvj/M/2MAtGxuyW34B2Q2CpYJwABcaDkAiCfDiMmx9bdyZ2wWpdHmdzjm3u/Rroim13efB7P4AYevEYADNKqweEaACPQb9gqWCcAAXGg5AIgklTo/MU5kGiYSl2PHl+4hb3k9jNAMGgMYFXHPtBdPf/YKlgnAAFxoOQCIM9lErwgYdHPIP0K1YolFqSA3sBROQAQ29LnBCVAQKFl2CpYJwABcaDkAiBZXZeLFhAvuPaehnADJNe6kbmwmBqGnYCxX+i74qzBXFhhAoVcMpcCRO2RXGdkBuXuhaNHXAZPXk+eGEsvsiCWOtnk2jm/RD+TRyB2Ze9/NOoohBjKgY1v3f9I7rOWv/1RDV+QjXwFEJmq0und6XT2DqjsuiYfVHbV5BZLeP9eAtHKAhpjdqViWGECieTDoE61ftVNUVj+yd1O8KDK396EN5xnCk2gWNsT4UKKXuC8EmvL4Wqzl8WDMfeeBqZQDH/YeXn55O+PvsdfG3lG1tlfmhltuf3qcbRSJF6pGgLq8XzWzATF3FWUlq9tAEQAeaeH`)
	aass(t)
	aa, err := types.DecodeBlock(a)
	assert.NoError(t, err)
	fmt.Println(aa)
	var blockA types.BlockHeader
	err = blockA.UnmarshalCBOR(bytes.NewReader(a))
	assert.NoError(t, err)
	fmt.Println(blockA)
}

func aass(t *testing.T) {
	params := miner.ReportConsensusFaultParams{}
	testutil.Provide(t, &params)
	fmt.Println(params.BlockHeader1, params.BlockHeader2)
	d, err := json.Marshal(params)
	assert.NoError(t, err)
	fmt.Println(string(d))

	md, err := actors.SerializeParams(&params)
	assert.NoError(t, err)
	fmt.Println(string(md))

	params2 := miner.ReportConsensusFaultParams{}
	assert.NoError(t, params2.UnmarshalCBOR(bytes.NewReader(md)))
	fmt.Println(params2)
}
