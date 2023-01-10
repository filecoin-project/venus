# Groups

* [Common](#common)
  * [AuthNew](#authnew)
  * [AuthVerify](#authverify)
  * [LogList](#loglist)
  * [LogSetLevel](#logsetlevel)
  * [Version](#version)
* [Strategy](#strategy)
  * [AddMethodIntoKeyBind](#addmethodintokeybind)
  * [AddMsgTypeIntoKeyBind](#addmsgtypeintokeybind)
  * [GetGroupByName](#getgroupbyname)
  * [GetKeyBindByName](#getkeybindbyname)
  * [GetKeyBinds](#getkeybinds)
  * [GetMethodTemplateByName](#getmethodtemplatebyname)
  * [GetMsgTypeTemplate](#getmsgtypetemplate)
  * [GetWalletTokenInfo](#getwallettokeninfo)
  * [GetWalletTokensByGroup](#getwallettokensbygroup)
  * [ListGroups](#listgroups)
  * [ListKeyBinds](#listkeybinds)
  * [ListMethodTemplates](#listmethodtemplates)
  * [ListMsgTypeTemplates](#listmsgtypetemplates)
  * [NewGroup](#newgroup)
  * [NewKeyBindCustom](#newkeybindcustom)
  * [NewKeyBindFromTemplate](#newkeybindfromtemplate)
  * [NewMethodTemplate](#newmethodtemplate)
  * [NewMsgTypeTemplate](#newmsgtypetemplate)
  * [NewStToken](#newsttoken)
  * [RemoveGroup](#removegroup)
  * [RemoveKeyBind](#removekeybind)
  * [RemoveKeyBindByAddress](#removekeybindbyaddress)
  * [RemoveMethodFromKeyBind](#removemethodfromkeybind)
  * [RemoveMethodTemplate](#removemethodtemplate)
  * [RemoveMsgTypeFromKeyBind](#removemsgtypefromkeybind)
  * [RemoveMsgTypeTemplate](#removemsgtypetemplate)
  * [RemoveStToken](#removesttoken)
* [StrategyVerify](#strategyverify)
  * [ContainWallet](#containwallet)
  * [ScopeWallet](#scopewallet)
  * [Verify](#verify)
* [Wallet](#wallet)
  * [WalletDelete](#walletdelete)
  * [WalletExport](#walletexport)
  * [WalletHas](#wallethas)
  * [WalletImport](#walletimport)
  * [WalletList](#walletlist)
  * [WalletNew](#walletnew)
  * [WalletSign](#walletsign)
* [WalletEvent](#walletevent)
  * [AddNewAddress](#addnewaddress)
  * [AddSupportAccount](#addsupportaccount)
* [WalletLock](#walletlock)
  * [Lock](#lock)
  * [LockState](#lockstate)
  * [SetPassword](#setpassword)
  * [Unlock](#unlock)
  * [VerifyPassword](#verifypassword)

## Common

### AuthNew


Perms: admin

Inputs:
```json
[
  [
    "write"
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
  "write"
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
  "APIVersion": 0
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
  "BindID": 0,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 0,
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
  "BindID": 0,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 0,
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
  "GroupID": 0,
  "Name": "string value",
  "KeyBinds": null
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
  "BindID": 0,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 0,
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
    "BindID": 0,
    "Name": "string value",
    "Address": "string value",
    "MetaTypes": 0,
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
  "MTId": 0,
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
  "MTTId": 0,
  "Name": "string value",
  "MetaTypes": 0
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
  "GroupID": 0,
  "Name": "string value",
  "KeyBinds": null
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
    "GroupID": 0,
    "Name": "string value",
    "KeyBinds": null
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
    "BindID": 0,
    "Name": "string value",
    "Address": "string value",
    "MetaTypes": 0,
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
    "MTId": 0,
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
    "MTTId": 0,
    "Name": "string value",
    "MetaTypes": 0
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
  "BindID": 0,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 0,
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
  "BindID": 0,
  "Name": "string value",
  "Address": "string value",
  "MetaTypes": 0,
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
      "/": "bafy2bzacebnkgxcy5pyk763pyw5l2sbltrai3qga5k2rcvvpgpdx2stlegnz4"
    },
    "Version": 42,
    "To": "f01234",
    "From": "f01234",
    "Nonce": 42,
    "Value": "0",
    "GasLimit": 0,
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
  "PrivateKey": null
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
    "PrivateKey": null
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

