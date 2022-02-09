# Groups

* [Common](#Common)
  * [AuthNew](#AuthNew)
  * [AuthVerify](#AuthVerify)
  * [LogList](#LogList)
  * [LogSetLevel](#LogSetLevel)
  * [Version](#Version)
* [Strategy](#Strategy)
  * [AddMethodIntoKeyBind](#AddMethodIntoKeyBind)
  * [AddMsgTypeIntoKeyBind](#AddMsgTypeIntoKeyBind)
  * [GetGroupByName](#GetGroupByName)
  * [GetKeyBindByName](#GetKeyBindByName)
  * [GetKeyBinds](#GetKeyBinds)
  * [GetMethodTemplateByName](#GetMethodTemplateByName)
  * [GetMsgTypeTemplate](#GetMsgTypeTemplate)
  * [GetWalletTokenInfo](#GetWalletTokenInfo)
  * [GetWalletTokensByGroup](#GetWalletTokensByGroup)
  * [ListGroups](#ListGroups)
  * [ListKeyBinds](#ListKeyBinds)
  * [ListMethodTemplates](#ListMethodTemplates)
  * [ListMsgTypeTemplates](#ListMsgTypeTemplates)
  * [NewGroup](#NewGroup)
  * [NewKeyBindCustom](#NewKeyBindCustom)
  * [NewKeyBindFromTemplate](#NewKeyBindFromTemplate)
  * [NewMethodTemplate](#NewMethodTemplate)
  * [NewMsgTypeTemplate](#NewMsgTypeTemplate)
  * [NewStToken](#NewStToken)
  * [RemoveGroup](#RemoveGroup)
  * [RemoveKeyBind](#RemoveKeyBind)
  * [RemoveKeyBindByAddress](#RemoveKeyBindByAddress)
  * [RemoveMethodFromKeyBind](#RemoveMethodFromKeyBind)
  * [RemoveMethodTemplate](#RemoveMethodTemplate)
  * [RemoveMsgTypeFromKeyBind](#RemoveMsgTypeFromKeyBind)
  * [RemoveMsgTypeTemplate](#RemoveMsgTypeTemplate)
  * [RemoveStToken](#RemoveStToken)
* [StrategyVerify](#StrategyVerify)
  * [ContainWallet](#ContainWallet)
  * [ScopeWallet](#ScopeWallet)
  * [Verify](#Verify)
* [Wallet](#Wallet)
  * [WalletDelete](#WalletDelete)
  * [WalletExport](#WalletExport)
  * [WalletHas](#WalletHas)
  * [WalletImport](#WalletImport)
  * [WalletList](#WalletList)
  * [WalletNew](#WalletNew)
  * [WalletSign](#WalletSign)
* [WalletEvent](#WalletEvent)
  * [AddNewAddress](#AddNewAddress)
  * [AddSupportAccount](#AddSupportAccount)
* [WalletLock](#WalletLock)
  * [Lock](#Lock)
  * [LockState](#LockState)
  * [SetPassword](#SetPassword)
  * [Unlock](#Unlock)
  * [VerifyPassword](#VerifyPassword)

## Common

### AuthNew


Perms: admin

Inputs:
```json
[
  [
    "string value"
  ]
]
```

Response: `"Ynl0ZSBhcnJheQ=="`

### AuthVerify
Auth


Perms: read

Inputs:
```json
[
  "string value"
]
```

Response:
```json
[
  "string value"
]
```

### LogList


Perms: read

Inputs: `[]`

Response:
```json
[
  "string value"
]
```

### LogSetLevel


Perms: write

Inputs:
```json
[
  "string value",
  "string value"
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
  "APIVersion": 131584
}
```

## Strategy

### AddMethodIntoKeyBind
AddMethodIntoKeyBind append methods into keyBind


Perms: admin

Inputs:
```json
[
  "string value",
  [
    "string value"
  ]
]
```

Response:
```json
{
  "BindID": 42,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 2,
  "Methods": [
    "string value"
  ]
}
```

### AddMsgTypeIntoKeyBind
AddMsgTypeIntoKeyBind append msgTypes into keyBind


Perms: admin

Inputs:
```json
[
  "string value",
  [
    123
  ]
]
```

Response:
```json
{
  "BindID": 42,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 2,
  "Methods": [
    "string value"
  ]
}
```

### GetGroupByName
GetGroupByName get a group by name


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
  "GroupID": 42,
  "Name": "string value",
  "KeyBinds": [
    {
      "BindID": 42,
      "Name": "string value",
      "Address": "string value",
      "MetaTypes": 2,
      "Methods": [
        "string value"
      ]
    }
  ]
}
```

### GetKeyBindByName
GetKeyBindByName get a keyBind by name


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
  "BindID": 42,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 2,
  "Methods": [
    "string value"
  ]
}
```

### GetKeyBinds
GetKeyBinds list keyBinds by address


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
    "BindID": 42,
    "Name": "string value",
    "Address": "string value",
    "MetaTypes": 2,
    "Methods": [
      "string value"
    ]
  }
]
```

### GetMethodTemplateByName
GetMethodTemplateByName get a method template by name


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
  "MTId": 42,
  "Name": "string value",
  "Methods": [
    "string value"
  ]
}
```

### GetMsgTypeTemplate
GetMsgTypeTemplate get a msgType template by name


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
  "MTTId": 42,
  "Name": "string value",
  "MetaTypes": 2
}
```

### GetWalletTokenInfo
GetWalletTokenInfo get group details by token


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
  "Token": "string value",
  "GroupID": 42,
  "Name": "string value",
  "KeyBinds": [
    {
      "BindID": 42,
      "Name": "string value",
      "Address": "string value",
      "MetaTypes": 2,
      "Methods": [
        "string value"
      ]
    }
  ]
}
```

### GetWalletTokensByGroup
GetWalletTokensByGroup list strategy tokens under the group


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response:
```json
[
  "string value"
]
```

### ListGroups
ListGroups list groups' simple information


Perms: admin

Inputs:
```json
[
  123,
  123
]
```

Response:
```json
[
  {
    "GroupID": 42,
    "Name": "string value",
    "KeyBinds": [
      {
        "BindID": 42,
        "Name": "string value",
        "Address": "string value",
        "MetaTypes": 2,
        "Methods": [
          "string value"
        ]
      }
    ]
  }
]
```

### ListKeyBinds
ListKeyBinds list keyBinds' details


Perms: admin

Inputs:
```json
[
  123,
  123
]
```

Response:
```json
[
  {
    "BindID": 42,
    "Name": "string value",
    "Address": "string value",
    "MetaTypes": 2,
    "Methods": [
      "string value"
    ]
  }
]
```

### ListMethodTemplates
ListMethodTemplates list method templates' details


Perms: admin

Inputs:
```json
[
  123,
  123
]
```

Response:
```json
[
  {
    "MTId": 42,
    "Name": "string value",
    "Methods": [
      "string value"
    ]
  }
]
```

### ListMsgTypeTemplates
ListMsgTypeTemplates list msgType templates' details


Perms: admin

Inputs:
```json
[
  123,
  123
]
```

Response:
```json
[
  {
    "MTTId": 42,
    "Name": "string value",
    "MetaTypes": 2
  }
]
```

### NewGroup
NewGroup create a group to group multiple keyBinds together


Perms: admin

Inputs:
```json
[
  "string value",
  [
    "string value"
  ]
]
```

Response: `{}`

### NewKeyBindCustom
NewKeyBindCustom create a keyBind with custom msyTypes and methods


Perms: admin

Inputs:
```json
[
  "string value",
  "f01234",
  [
    123
  ],
  [
    "string value"
  ]
]
```

Response: `{}`

### NewKeyBindFromTemplate
NewKeyBindFromTemplate create a keyBind form msgType template and method template


Perms: admin

Inputs:
```json
[
  "string value",
  "f01234",
  "string value",
  "string value"
]
```

Response: `{}`

### NewMethodTemplate
NewMethodTemplate create a method template


Perms: admin

Inputs:
```json
[
  "string value",
  [
    "string value"
  ]
]
```

Response: `{}`

### NewMsgTypeTemplate
NewMsgTypeTemplate create a msgType template


Perms: admin

Inputs:
```json
[
  "string value",
  [
    123
  ]
]
```

Response: `{}`

### NewStToken
NewStToken generate a random token from group


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `"string value"`

### RemoveGroup
RemoveGroup delete group by name


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### RemoveKeyBind
RemoveKeyBind delete keyBind by name


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### RemoveKeyBindByAddress
RemoveKeyBindByAddress delete some keyBinds by address


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `9`

### RemoveMethodFromKeyBind
RemoveMethodFromKeyBind remove methods from keyBind


Perms: admin

Inputs:
```json
[
  "string value",
  [
    "string value"
  ]
]
```

Response:
```json
{
  "BindID": 42,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 2,
  "Methods": [
    "string value"
  ]
}
```

### RemoveMethodTemplate
RemoveMethodTemplate delete method template by name


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### RemoveMsgTypeFromKeyBind
RemoveMsgTypeFromKeyBind remove msgTypes form keyBind


Perms: admin

Inputs:
```json
[
  "string value",
  [
    123
  ]
]
```

Response:
```json
{
  "BindID": 42,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 2,
  "Methods": [
    "string value"
  ]
}
```

### RemoveMsgTypeTemplate
RemoveMsgTypeTemplate delete msgType template by name


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### RemoveStToken
RemoveStToken delete strategy token


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

## StrategyVerify

### ContainWallet
ContainWallet Check if it is visible to the wallet


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `true`

### ScopeWallet
ScopeWallet get the wallet scope


Perms: admin

Inputs: `[]`

Response:
```json
{
  "Root": true,
  "Addresses": [
    "f01234"
  ]
}
```

### Verify
Verify verify the address strategy permissions


Perms: admin

Inputs:
```json
[
  "f01234",
  "message",
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
  }
]
```

Response: `{}`

## Wallet

### WalletDelete


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `{}`

### WalletExport


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
  "Type": "bls",
  "PrivateKey": "Ynl0ZSBhcnJheQ=="
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

### WalletImport


Perms: admin

Inputs:
```json
[
  {
    "Type": "bls",
    "PrivateKey": "Ynl0ZSBhcnJheQ=="
  }
]
```

Response: `"f01234"`

### WalletList


Perms: read

Inputs: `[]`

Response:
```json
[
  "f01234"
]
```

### WalletNew


Perms: admin

Inputs:
```json
[
  "bls"
]
```

Response: `"f01234"`

### WalletSign


Perms: sign

Inputs:
```json
[
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

## WalletEvent

### AddNewAddress


Perms: admin

Inputs:
```json
[
  [
    "f01234"
  ]
]
```

Response: `{}`

### AddSupportAccount


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

## WalletLock

### Lock
lock the wallet and disable IWallet logic


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### LockState
show lock state


Perms: admin

Inputs: `[]`

Response: `true`

### SetPassword
SetPassword do it first after program setup


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### Unlock
unlock the wallet and enable IWallet logic


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

### VerifyPassword
VerifyPassword verify that the passwords are consistent


Perms: admin

Inputs:
```json
[
  "string value"
]
```

Response: `{}`

