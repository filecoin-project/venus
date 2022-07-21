# Groups

* [Messager](#Messager)
  * [ActiveAddress](#ActiveAddress)
  * [ClearUnFillMessage](#ClearUnFillMessage)
  * [DeleteAddress](#DeleteAddress)
  * [DeleteNode](#DeleteNode)
  * [ForbiddenAddress](#ForbiddenAddress)
  * [ForcePushMessage](#ForcePushMessage)
  * [ForcePushMessageWithId](#ForcePushMessageWithId)
  * [GetAddress](#GetAddress)
  * [GetMessageByFromAndNonce](#GetMessageByFromAndNonce)
  * [GetMessageBySignedCid](#GetMessageBySignedCid)
  * [GetMessageByUid](#GetMessageByUid)
  * [GetMessageByUnsignedCid](#GetMessageByUnsignedCid)
  * [GetNode](#GetNode)
  * [GetSharedParams](#GetSharedParams)
  * [HasAddress](#HasAddress)
  * [HasMessageByUid](#HasMessageByUid)
  * [HasNode](#HasNode)
  * [ListAddress](#ListAddress)
  * [ListBlockedMessage](#ListBlockedMessage)
  * [ListFailedMessage](#ListFailedMessage)
  * [ListMessage](#ListMessage)
  * [ListMessageByAddress](#ListMessageByAddress)
  * [ListMessageByFromState](#ListMessageByFromState)
  * [ListNode](#ListNode)
  * [MarkBadMessage](#MarkBadMessage)
  * [PushMessage](#PushMessage)
  * [PushMessageWithId](#PushMessageWithId)
  * [RecoverFailedMsg](#RecoverFailedMsg)
  * [RefreshSharedParams](#RefreshSharedParams)
  * [ReplaceMessage](#ReplaceMessage)
  * [RepublishMessage](#RepublishMessage)
  * [SaveNode](#SaveNode)
  * [Send](#Send)
  * [SetFeeParams](#SetFeeParams)
  * [SetLogLevel](#SetLogLevel)
  * [SetSelectMsgNum](#SetSelectMsgNum)
  * [SetSharedParams](#SetSharedParams)
  * [UpdateAllFilledMessage](#UpdateAllFilledMessage)
  * [UpdateFilledMessageByID](#UpdateFilledMessageByID)
  * [UpdateMessageStateByID](#UpdateMessageStateByID)
  * [UpdateNonce](#UpdateNonce)
  * [WaitMessage](#WaitMessage)
  * [WalletHas](#WalletHas)

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

### ForcePushMessage


Perms: admin

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

### ForcePushMessageWithId


Perms: write

Inputs:
```json
[
  "string value",
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
  "selMsgNum": 42,
  "state": 1,
  "gasOverEstimation": 12.3,
  "maxFee": "0",
  "gasFeeCap": "0",
  "gasOverPremium": 12.3,
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
  "FromUser": "test",
  "State": 1,
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
  "FromUser": "test",
  "State": 1,
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
  "FromUser": "test",
  "State": 1,
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
  "FromUser": "test",
  "State": 1,
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
  "Type": 2
}
```

### GetSharedParams


Perms: admin

Inputs: `[]`

Response:
```json
{
  "id": 42,
  "gasOverEstimation": 12.3,
  "maxFee": "0",
  "gasFeeCap": "0",
  "gasOverPremium": 12.3,
  "selMsgNum": 42
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
    "selMsgNum": 42,
    "state": 1,
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
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
    "FromUser": "test",
    "State": 1,
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
    "FromUser": "test",
    "State": 1,
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
    "FromUser": "test",
    "State": 1,
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
    "FromUser": "test",
    "State": 1,
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
    "FromUser": "test",
    "State": 1,
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
    "Type": 2
  }
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

### RefreshSharedParams


Perms: admin

Inputs: `[]`

Response: `{}`

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
    "Type": 2
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
  "f01234",
  12.3,
  12.3,
  "string value",
  "string value"
]
```

Response: `{}`

### SetLogLevel


Perms: admin

Inputs:
```json
[
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
    "gasOverEstimation": 12.3,
    "maxFee": "0",
    "gasFeeCap": "0",
    "gasOverPremium": 12.3,
    "selMsgNum": 42
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
  "FromUser": "test",
  "State": 1,
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

