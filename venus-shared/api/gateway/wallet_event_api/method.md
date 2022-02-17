# Groups

* [WalletEvent](#WalletEvent)
  * [AddNewAddress](#AddNewAddress)
  * [ListenWalletEvent](#ListenWalletEvent)
  * [RemoveAddress](#RemoveAddress)
  * [ResponseWalletEvent](#ResponseWalletEvent)
  * [SupportNewAccount](#SupportNewAccount)

## WalletEvent

### AddNewAddress


Perms: write

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

### ListenWalletEvent


Perms: write

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


Perms: write

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


Perms: write

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


Perms: write

Inputs:
```json
[
  "e26f1e5c-47f7-4561-a11d-18fab6e748af",
  "string value"
]
```

Response: `{}`

