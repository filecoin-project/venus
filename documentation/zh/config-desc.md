# 配置文件解析

```json
{
	"api": {
		"venusAuthURL": "http://127.0.0.1:8989",
		"venusAuthToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJuYW1lIjoiZWx2aW4iLCJwZXJtIjoiYWRtaW4iLCJleHQiOiIifQ.bs6_hirDtHP8hX46xpr2VAoQcgtSLNagmPDniifX6aA",
		"apiAddress": "/ip4/127.0.0.1/tcp/3453",
		"accessControlAllowOrigin": [
			"http://localhost:8080",
			"https://localhost:8080",
			"http://127.0.0.1:8080",
			"https://127.0.0.1:8080"
		],
		"accessControlAllowCredentials": false,
		"accessControlAllowMethods": [
			"GET",
			"POST",
			"PUT"
		]
	},
	"bootstrap": {
		"addresses": [],
		"period": "30s"
	},
	"datastore": {
		"type": "badgerds",
		"path": "badger"
	},
	"mpool": {
		"maxNonceGap": 100,
		"maxFee": "10 FIL"
	},
	"parameters": {
		"networkType": 2, //网络类型，1:主网，2：2k，4：cali测试网
		"allowableClockDriftSecs": 1 // 系统允许未来多长时间的区块，单位秒
	},
	"observability": {
		"metrics": {
			"prometheusEnabled": false,
			"reportInterval": "5s",
			"prometheusEndpoint": "/ip4/0.0.0.0/tcp/9400"
		},
		"tracing": {
			"jaegerTracingEnabled": false,
			"probabilitySampler": 1,
			"jaegerEndpoint": "localhost:6831",
			"servername": "venus-node"
		}
	},
	"swarm": {
		"address": "/ip4/0.0.0.0/tcp/0"
	},
	"walletModule": {
		"defaultAddress": "\u003cempty\u003e",
		"passphraseConfig": {
			"scryptN": 2097152,
			"scryptP": 1
		},
		"remoteEnable": false, //是否支持远程wallet
		"remoteBackend": "" //远程wallet的ip地址
	},
	"slashFilter": {
		"type": "local", //两种：local或者mysql
		"mysql": {
			"connectionString": "",
			"maxOpenConn": 0,
			"maxIdleConn": 0,
			"connMaxLifeTime": 0,
			"debug": false
		}
	},
	"rateLimit": { //需要配合auth服务一起设置才行
		"RedisEndpoint": "",
		"user": "",
		"pwd": "",
		"enable": false
	},
	"fevm": {
		"enableEthRPC": false,
		"ethTxHashMappingLifetimeDays": 0,
		"event": {
			"enableRealTimeFilterAPI": false,
			"enableHistoricFilterAPI": false,
			"filterTTL": "24h0m0s",
			"maxFilters": 100,
			"maxFilterResults": 10000,
			"maxFilterHeightRange": 2880,
			"databasePath": ""
		}
	}
}
```