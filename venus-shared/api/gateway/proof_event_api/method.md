# Groups

* [ProofEvent](#ProofEvent)
  * [ListenProofEvent](#ListenProofEvent)
  * [ResponseProofEvent](#ResponseProofEvent)

## ProofEvent

### ListenProofEvent


Perms: write

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

