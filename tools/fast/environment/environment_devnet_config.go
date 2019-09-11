package environment

var devnetConfigs = map[string]DevnetConfig{
	"user": {
		Name:            "user",
		GenesisLocation: "https://genesis.user.kittyhawk.wtf/genesis.car",
		FaucetTap:       "https://faucet.user.kittyhawk.wtf/tap",
	},
	"nightly": {
		Name:            "nightly",
		GenesisLocation: "https://genesis.nightly.kittyhawk.wtf/genesis.car",
		FaucetTap:       "https://faucet.nightly.kittyhawk.wtf/tap",
	},
	"staging": {
		Name:            "staging",
		GenesisLocation: "https://genesis.staging.kittyhawk.wtf/genesis.car",
		FaucetTap:       "https://faucet.staging.kittyhawk.wtf/tap",
	},
}
