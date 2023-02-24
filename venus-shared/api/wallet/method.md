# Sample code of curl

```bash
# <Inputs> corresponding to the value of Inputs Tag of each API
curl http://<ip>:<port>/rpc/v0 -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer <token>"  -d '{"method": "Filecoin.<method>", "params": <Inputs>, "id": 0}'
```
# Groups

* [Common](#common)
  * [AuthNew](#authnew)
  * [AuthVerify](#authverify)
  * [ListSignedRecord](#listsignedrecord)
  * [LogList](#loglist)
  * [LogSetLevel](#logsetlevel)
  * [Version](#version)
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

### ListSignedRecord


Perms: read

Inputs:
```json
[
  {
    "ID": "string value",
    "Type": "message",
    "Signer": "f01234",
    "IsError": true,
    "Skip": 123,
    "Limit": 123,
    "After": "0001-01-01T00:00:00Z",
    "Before": "0001-01-01T00:00:00Z"
  }
]
```

Response:
```json
[
  {
    "ID": "string value",
    "Type": "message",
    "Signer": "f01234",
    "Err": {},
    "RawMsg": "Ynl0ZSBhcnJheQ==",
    "Signature": {
      "Type": 2,
      "Data": "Ynl0ZSBhcnJheQ=="
    },
    "CreateAt": "0001-01-01T00:00:00Z"
  }
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
  "APIVersion": 131840
}
```

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

