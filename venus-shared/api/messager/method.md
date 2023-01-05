# Groups

* [Messager](#messager)
  * [ActiveAddress](#activeaddress)
  * [ClearUnFillMessage](#clearunfillmessage)
  * [DeleteAddress](#deleteaddress)
  * [DeleteNode](#deletenode)
  * [ForbiddenAddress](#forbiddenaddress)
  * [GetActorCfgByID](#getactorcfgbyid)
  * [GetAddress](#getaddress)
  * [GetMessageByFromAndNonce](#getmessagebyfromandnonce)
  * [GetMessageBySignedCid](#getmessagebysignedcid)
  * [GetMessageByUid](#getmessagebyuid)
  * [GetMessageByUnsignedCid](#getmessagebyunsignedcid)
  * [GetNode](#getnode)
  * [GetSharedParams](#getsharedparams)
  * [HasAddress](#hasaddress)
  * [HasMessageByUid](#hasmessagebyuid)
  * [HasNode](#hasnode)
  * [ListActorCfg](#listactorcfg)
  * [ListAddress](#listaddress)
  * [ListBlockedMessage](#listblockedmessage)
  * [ListFailedMessage](#listfailedmessage)
  * [ListMessage](#listmessage)
  * [ListMessageByAddress](#listmessagebyaddress)
  * [ListMessageByFromState](#listmessagebyfromstate)
  * [ListNode](#listnode)
  * [LogList](#loglist)
  * [MarkBadMessage](#markbadmessage)
  * [NetAddrsListen](#netaddrslisten)
  * [NetConnect](#netconnect)
  * [NetFindPeer](#netfindpeer)
  * [NetPeers](#netpeers)
  * [PushMessage](#pushmessage)
  * [PushMessageWithId](#pushmessagewithid)
  * [RecoverFailedMsg](#recoverfailedmsg)
  * [ReplaceMessage](#replacemessage)
  * [RepublishMessage](#republishmessage)
  * [SaveActorCfg](#saveactorcfg)
  * [SaveNode](#savenode)
  * [Send](#send)
  * [SetFeeParams](#setfeeparams)
  * [SetLogLevel](#setloglevel)
  * [SetSelectMsgNum](#setselectmsgnum)
  * [SetSharedParams](#setsharedparams)
  * [UpdateActorCfg](#updateactorcfg)
  * [UpdateAllFilledMessage](#updateallfilledmessage)
  * [UpdateFilledMessageByID](#updatefilledmessagebyid)
  * [UpdateMessageStateByID](#updatemessagestatebyid)
  * [UpdateNonce](#updatenonce)
  * [Version](#version)
  * [WaitMessage](#waitmessage)
  * [WalletHas](#wallethas)

## Messager

### ActiveAddress


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### ClearUnFillMessage


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `123`

### DeleteAddress


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### DeleteNode


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### ForbiddenAddress


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### GetActorCfgByID


Perms: read

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af"
]
```

Response:
```json
{
  "id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "addr": 17,
  "codeCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "method": 1,
  "gasOverEstimation": 12.3,
  "maxFee": "0",
  "gasFeeCap": "0",
  "gasOverPremium": 12.3,
  "baseFee": "0",
  "createAt": "0001-01-01T00:00:00Z",
  "updateAt": "0001-01-01T00:00:00Z"
}
```

### GetAddress


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
  "id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "addr": "f01234",
  "nonce": 42,
  "weight": 9,
  "state": 1,
  "selMsgNum": 42,
  "gasOverEstimation": 12.3,
  "maxFee": "0",
  "gasFeeCap": "0",
  "gasOverPremium": 12.3,
  "baseFee": "0",
  "isDeleted": 123,
  "createAt": "0001-01-01T00:00:00Z",
  "updateAt": "0001-01-01T00:00:00Z"
}
```

### GetMessageByFromAndNonce


Perms: read

Inputs:
```json
[
  "f01234",
  42
]
```

Response:
```json
{
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "UnsignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ==",
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Height": 100,
  "Confidence": 10,
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9
  },
  "TipSetKey": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Meta": {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  },
  "WalletName": "test",
  "State": 1,
  "ErrorMsg": "",
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### GetMessageBySignedCid


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "UnsignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ==",
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Height": 100,
  "Confidence": 10,
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9
  },
  "TipSetKey": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Meta": {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  },
  "WalletName": "test",
  "State": 1,
  "ErrorMsg": "",
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### GetMessageByUid


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response:
```json
{
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "UnsignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ==",
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Height": 100,
  "Confidence": 10,
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9
  },
  "TipSetKey": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Meta": {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  },
  "WalletName": "test",
  "State": 1,
  "ErrorMsg": "",
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### GetMessageByUnsignedCid


Perms: read

Inputs:
```json
[
  {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  }
]
```

Response:
```json
{
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "UnsignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ==",
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Height": 100,
  "Confidence": 10,
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9
  },
  "TipSetKey": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Meta": {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  },
  "WalletName": "test",
  "State": 1,
  "ErrorMsg": "",
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### GetNode


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
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "Name": "venus",
  "URL": "/ip4/127.0.0.1/tcp/3453",
  "Token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0._eHBJJAiBzQmfcbD_vVmtTrkgyJQ-LOgGOiHfb8rU1I",
  "Type": 2,
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### GetSharedParams


Perms: admin

Inputs: `[]`

Response:
```json
{
  "id": 42,
  "selMsgNum": 42,
  "gasOverEstimation": 12.3,
  "maxFee": "0",
  "gasFeeCap": "0",
  "gasOverPremium": 12.3,
  "baseFee": "0"
}
```

### HasAddress


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### HasMessageByUid


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response: `true`

### HasNode


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `true`

### ListActorCfg


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "addr": 17,
    "codeCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "method": 1,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "baseFee": "0",
    "createAt": "0001-01-01T00:00:00Z",
    "updateAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListAddress


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "addr": "f01234",
    "nonce": 42,
    "weight": 9,
    "state": 1,
    "selMsgNum": 42,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "baseFee": "0",
    "isDeleted": 123,
    "createAt": "0001-01-01T00:00:00Z",
    "updateAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListBlockedMessage


Perms: admin

Inputs:
```json
[
  "f01234",
  60000000000
]
```

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "UnsignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Height": 100,
    "Confidence": 10,
    "Receipt": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "GasUsed": 9
    },
    "TipSetKey": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Meta": {
      "expireEpoch": 10101,
      "gasOverEstimation": 12.3,
      "maxFee": "0",
      "gasOverPremium": 12.3
    },
    "WalletName": "test",
    "State": 1,
    "ErrorMsg": "",
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListFailedMessage


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "UnsignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Height": 100,
    "Confidence": 10,
    "Receipt": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "GasUsed": 9
    },
    "TipSetKey": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Meta": {
      "expireEpoch": 10101,
      "gasOverEstimation": 12.3,
      "maxFee": "0",
      "gasOverPremium": 12.3
    },
    "WalletName": "test",
    "State": 1,
    "ErrorMsg": "",
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListMessage


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "UnsignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Height": 100,
    "Confidence": 10,
    "Receipt": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "GasUsed": 9
    },
    "TipSetKey": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Meta": {
      "expireEpoch": 10101,
      "gasOverEstimation": 12.3,
      "maxFee": "0",
      "gasOverPremium": 12.3
    },
    "WalletName": "test",
    "State": 1,
    "ErrorMsg": "",
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListMessageByAddress


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "UnsignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Height": 100,
    "Confidence": 10,
    "Receipt": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "GasUsed": 9
    },
    "TipSetKey": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Meta": {
      "expireEpoch": 10101,
      "gasOverEstimation": 12.3,
      "maxFee": "0",
      "gasOverPremium": 12.3
    },
    "WalletName": "test",
    "State": 1,
    "ErrorMsg": "",
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListMessageByFromState


Perms: admin

Inputs:
```json
[
  "f01234",
  3,
  true,
  123,
  123
]
```

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "UnsignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "SignedCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "Height": 100,
    "Confidence": 10,
    "Receipt": {
      "ExitCode": 0,
      "Return": "Ynl0ZSBhcnJheQ==",
      "GasUsed": 9
    },
    "TipSetKey": [
      {
        "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
      },
      {
        "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
      }
    ],
    "Meta": {
      "expireEpoch": 10101,
      "gasOverEstimation": 12.3,
      "maxFee": "0",
      "gasOverPremium": 12.3
    },
    "WalletName": "test",
    "State": 1,
    "ErrorMsg": "",
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### ListNode


Perms: admin

Inputs: `[]`

Response:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Name": "venus",
    "URL": "/ip4/127.0.0.1/tcp/3453",
    "Token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0._eHBJJAiBzQmfcbD_vVmtTrkgyJQ-LOgGOiHfb8rU1I",
    "Type": 2,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

### LogList


Perms: write

Inputs: `[]`

Response:
```json
[
  "string value"
]
```

### MarkBadMessage


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### NetAddrsListen


Perms: read

Inputs: `[]`

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Addrs": [
    "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
  ]
}
```

### NetConnect


Perms: admin

Inputs:
```json
[
  {
    "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Addrs": [
      "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
    ]
  }
]
```

Response: `{}`

### NetFindPeer


Perms: read

Inputs:
```json
[
  "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf"
]
```

Response:
```json
{
  "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
  "Addrs": [
    "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
  ]
}
```

### NetPeers


Perms: read

Inputs: `[]`

Response:
```json
[
  {
    "ID": "12D3KooWGzxzKZYveHXtpG6AsrUJBcWxHBFS2HsEoGTxrMLvKXtf",
    "Addrs": [
      "/ip4/52.36.61.156/tcp/1347/p2p/12D3KooWFETiESTf1v4PGUvtnxMAcEFMzLZbJGg4tjWfGEimYior"
    ]
  }
]
```

### PushMessage


Perms: write

Inputs:
```json
[
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  }
]
```

Response: `"string value"`

### PushMessageWithId


Perms: write

Inputs:
```json
[
  "string value",
  {
    "CID": {
      "/": "bafy2bzacebbpdegvr3i4cosewthysg5xkxpqfn2wfcz6mv2hmoktwbdxkax4s"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 9,
    "GasFeeCap": "0",
    "GasPremium": "0",
    "Method": 1,
    "Params": "Ynl0ZSBhcnJheQ=="
  },
  {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  }
]
```

Response: `"string value"`

### RecoverFailedMsg


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response:
```json
[
  "string value"
]
```

### ReplaceMessage


Perms: admin

Inputs:
```json
[
  {
    "ID": "string value",
    "Auto": true,
    "MaxFee": "0",
    "GasLimit": 9,
    "GasPremium": "0",
    "GasFeecap": "0",
    "GasOverPremium": 12.3
  }
]
```

Response:
```json
{
  "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
}
```

### RepublishMessage


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### SaveActorCfg


Perms: admin

Inputs:
```json
[
  {
    "id": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "addr": 17,
    "codeCid": {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    "method": 1,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "baseFee": "0",
    "createAt": "0001-01-01T00:00:00Z",
    "updateAt": "0001-01-01T00:00:00Z"
  }
]
```

Response: `{}`

### SaveNode


Perms: admin

Inputs:
```json
[
  {
    "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
    "Name": "venus",
    "URL": "/ip4/127.0.0.1/tcp/3453",
    "Token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJBbGxvdyI6WyJyZWFkIiwid3JpdGUiLCJzaWduIiwiYWRtaW4iXX0._eHBJJAiBzQmfcbD_vVmtTrkgyJQ-LOgGOiHfb8rU1I",
    "Type": 2,
    "CreatedAt": "0001-01-01T00:00:00Z",
    "UpdatedAt": "0001-01-01T00:00:00Z"
  }
]
```

Response: `{}`

### Send


Perms: admin

Inputs:
```json
[
  {
    "To": "f01234",
    "From": "f01234",
    "Val": "0",
    "Account": "string value",
    "GasPremium": "0",
    "GasFeeCap": "0",
    "GasLimit": 10000,
    "Method": 1,
    "Params": "string value",
    "ParamsType": "json"
  }
]
```

Response: `"string value"`

### SetFeeParams


Perms: admin

Inputs:
```json
[
  {
    "address": "f01234",
    "gasOverEstimation": 12.3,
    "gasOverPremium": 12.3,
    "maxFeeStr": "string value",
    "gasFeeCapStr": "string value",
    "baseFeeStr": "string value"
  }
]
```

Response: `{}`

### SetLogLevel


Perms: admin

Inputs:
```json
[
  "string value",
  "string value"
]
```

Response: `{}`

### SetSelectMsgNum


Perms: admin

Inputs:
```json
[
  "f01234",
  42
]
```

Response: `{}`

### SetSharedParams


Perms: admin

Inputs:
```json
[
  {
    "id": 42,
    "selMsgNum": 42,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "baseFee": "0"
  }
]
```

Response: `{}`

### UpdateActorCfg


Perms: admin

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  {
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "baseFee": "0"
  }
]
```

Response: `{}`

### UpdateAllFilledMessage


Perms: admin

Inputs: `[]`

Response: `123`

### UpdateFilledMessageByID


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `"string value"`

### UpdateMessageStateByID


Perms: admin

Inputs:
```json
[
  "string value",
  3
]
```

Response: `{}`

### UpdateNonce


Perms: admin

Inputs:
```json
[
  "f01234",
  42
]
```

Response: `{}`

### Version
Version provides information about API provider


Perms: read

Inputs: `[]`

Response:
```json
{
  "Version": "string value",
  "APIVersion": 131840
}
```

### WaitMessage


Perms: read

Inputs:
```json
[
  "string value",
  42
]
```

Response:
```json
{
  "ID": "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "UnsignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "SignedCid": {
    "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
  },
  "Version": 42,
  "To": "f01234",
  "From": "f01234",
  "Nonce": 42,
  "Value": "0",
  "GasLimit": 9,
  "GasFeeCap": "0",
  "GasPremium": "0",
  "Method": 1,
  "Params": "Ynl0ZSBhcnJheQ==",
  "Signature": {
    "Type": 2,
    "Data": "Ynl0ZSBhcnJheQ=="
  },
  "Height": 100,
  "Confidence": 10,
  "Receipt": {
    "ExitCode": 0,
    "Return": "Ynl0ZSBhcnJheQ==",
    "GasUsed": 9
  },
  "TipSetKey": [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ],
  "Meta": {
    "expireEpoch": 10101,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasOverPremium": 12.3
  },
  "WalletName": "test",
  "State": 1,
  "ErrorMsg": "",
  "CreatedAt": "0001-01-01T00:00:00Z",
  "UpdatedAt": "0001-01-01T00:00:00Z"
}
```

### WalletHas


Perms: read

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

