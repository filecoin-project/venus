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
  "Id": "07070707-0707-0707-0707-070707070707",
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
    "Id": "07070707-0707-0707-0707-070707070707",
    "Payload": "Ynl0ZSBhcnJheQ==",
    "Error": "string value"
  }
]
```

Response: `{}`

