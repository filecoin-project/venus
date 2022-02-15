# Groups

* [MarketEvent](#MarketEvent)
  * [IsUnsealed](#IsUnsealed)
  * [ListMarketConnectionsState](#ListMarketConnectionsState)
  * [ListenMarketEvent](#ListenMarketEvent)
  * [ResponseMarketEvent](#ResponseMarketEvent)
  * [SectorsUnsealPiece](#SectorsUnsealPiece)
* [ProofEvent](#ProofEvent)
  * [ComputeProof](#ComputeProof)
  * [ListConnectedMiners](#ListConnectedMiners)
  * [ListMinerConnection](#ListMinerConnection)
  * [ListenProofEvent](#ListenProofEvent)
  * [ResponseProofEvent](#ResponseProofEvent)
* [WalletEvent](#WalletEvent)
  * [AddNewAddress](#AddNewAddress)
  * [ListWalletInfo](#ListWalletInfo)
  * [ListWalletInfoByWallet](#ListWalletInfoByWallet)
  * [ListenWalletEvent](#ListenWalletEvent)
  * [RemoveAddress](#RemoveAddress)
  * [ResponseWalletEvent](#ResponseWalletEvent)
  * [SupportNewAccount](#SupportNewAccount)
  * [WalletHas](#WalletHas)
  * [WalletSign](#WalletSign)

## MarketEvent

### IsUnsealed


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "ID": {
      "Miner": 1000,
      "Number": 9
    },
    "ProofType": 8
  },
  10,
  1032
]
```

Response: `true`

### ListMarketConnectionsState


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Addr": "f01234",
    "Conn": {
      "Connections": [
        {
          "Addrs": [
            "f01234"
          ],
          "ChannelId": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
          "Ip": "string value",
          "RequestCount": 123,
          "CreateTime": "0001-01-01T00:00:00Z"
        }
      ],
      "ConnectionCount": 123
    }
  }
]
```

### ListenMarketEvent


Perms: read

Inputs:
```json
[
  {
    "Miner": "f01234"
  }
]
```

Response:
```json
{
  "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "Method": "string value",
  "Payload": "Ynl0ZSBhcnJheQ=="
}
```

### ResponseMarketEvent


Perms: read

Inputs:
```json
[
  {
    "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

### SectorsUnsealPiece


Perms: admin

Inputs:
```json
[
  "f01234",
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  {
    "ID": {
      "Miner": 1000,
      "Number": 9
    },
    "ProofType": 8
  },
  10,
  1032,
  "string value"
]
```

Response: `{}`

## ProofEvent

### ComputeProof


Perms: admin

Inputs:
```json
[
  "f01234",
  [
    {
      "SealProof": 8,
      "SectorNumber": 9,
      "SectorKey": null,
      "SealedCID": {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      }
    }
  ],
  "Bw==",
  10101,
  15
]
```

Response:
```json
[
  {
    "PoStProof": 8,
    "ProofBytes": "Ynl0ZSBhcnJheQ=="
  }
]
```

### ListConnectedMiners


Perms: admin

Inputs: `[]`

Response:
```json
[
  "f01234"
]
```

### ListMinerConnection


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
{
  "Connections": [
    {
      "Addrs": [
        "f01234"
      ],
      "ChannelId": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
      "Ip": "string value",
      "RequestCount": 123,
      "CreateTime": "0001-01-01T00:00:00Z"
    }
  ],
  "ConnectionCount": 123
}
```

### ListenProofEvent


Perms: read

Inputs:
```json
[
  {
    "MinerAddress": "f01234"
  }
]
```

Response:
```json
{
  "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "Method": "string value",
  "Payload": "Ynl0ZSBhcnJheQ=="
}
```

### ResponseProofEvent


Perms: read

Inputs:
```json
[
  {
    "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

## WalletEvent

### AddNewAddress


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  [
    "f01234"
  ]
]
```

Response: `{}`

### ListWalletInfo


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "Account": "string value",
    "SupportAccounts": [
      "string value"
    ],
    "ConnectStates": [
      {
        "Addrs": [
          "f01234"
        ],
        "ChannelId": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
        "Ip": "string value",
        "RequestCount": 123,
        "CreateTime": "0001-01-01T00:00:00Z"
      }
    ]
  }
]
```

### ListWalletInfoByWallet


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response:
```json
{
  "Account": "string value",
  "SupportAccounts": [
    "string value"
  ],
  "ConnectStates": [
    {
      "Addrs": [
        "f01234"
      ],
      "ChannelId": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
      "Ip": "string value",
      "RequestCount": 123,
      "CreateTime": "0001-01-01T00:00:00Z"
    }
  ]
}
```

### ListenWalletEvent


Perms: read

Inputs:
```json
[
  {
    "SupportAccounts": [
      "string value"
    ],
    "SignBytes": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response:
```json
{
  "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "Method": "string value",
  "Payload": "Ynl0ZSBhcnJheQ=="
}
```

### RemoveAddress


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  [
    "f01234"
  ]
]
```

Response: `{}`

### ResponseWalletEvent


Perms: read

Inputs:
```json
[
  {
    "Id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

### SupportNewAccount


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "string value"
]
```

Response: `{}`

### WalletHas


Perms: admin

Inputs:
```json
[
  "string value",
  "f01234"
]
```

Response: `true`

### WalletSign


Perms: admin

Inputs:
```json
[
  "string value",
  "f01234",
  "Ynl0ZSBhcnJheQ==",
  {
    "Type": "message",
    "Extra": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response:
```json
{
  "Type": 2,
  "Data": "Ynl0ZSBhcnJheQ=="
}
```

