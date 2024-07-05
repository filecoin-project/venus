# Sample code of curl

```bash
# <Inputs> corresponding to the value of Inputs Tag of each API
curl http://<ip>:<port>/rpc/v0 -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer <token>"  -d '{"method": "F3.<method>", "params": <Inputs>, "id": 0}'
```
# Groups

* [F3](#f3)
  * [F3GetCertificate](#f3getcertificate)
  * [F3GetLatestCertificate](#f3getlatestcertificate)
  * [F3Participate](#f3participate)

## F3

### F3GetCertificate
F3GetCertificate returns a finality certificate at given instance number


Perms: read

Inputs:
```json
[
  42
]
```

Response:
```json
{
  "GPBFTInstance": 0,
  "ECChain": null,
  "SupplementalData": {
    "Commitments": [
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0
    ],
    "PowerTable": null
  },
  "Signers": [
    0
  ],
  "Signature": null,
  "PowerTableDelta": null
}
```

### F3GetLatestCertificate
F3GetLatestCertificate returns the latest finality certificate


Perms: read

Inputs: `[]`

Response:
```json
{
  "GPBFTInstance": 0,
  "ECChain": null,
  "SupplementalData": {
    "Commitments": [
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0,
      0
    ],
    "PowerTable": null
  },
  "Signers": [
    0
  ],
  "Signature": null,
  "PowerTableDelta": null
}
```

### F3Participate
F3Participate should be called by a miner node to participate in signing F3 consensus.
The address should be of type ID
The returned channel will never be closed by the F3
If it is closed without the context being cancelled, the caller should retry.
The values returned on the channel will inform the caller about participation
Empty strings will be sent if participation succeeded, non-empty strings explain possible errors.


Perms: admin

Inputs:
```json
[
  "f01234"
]
```

Response: `"string value"`

