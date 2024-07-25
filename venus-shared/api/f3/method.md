# Sample code of curl

```bash
# <Inputs> corresponding to the value of Inputs Tag of each API
curl http://<ip>:<port>/rpc/v0 -X POST -H "Content-Type: application/json"  -H "Authorization: Bearer <token>"  -d '{"method": "F3.<method>", "params": <Inputs>, "id": 0}'
```
# Groups

* [F3](#f3)
  * [F3GetCertificate](#f3getcertificate)
  * [F3GetECPowerTable](#f3getecpowertable)
  * [F3GetF3PowerTable](#f3getf3powertable)
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

### F3GetECPowerTable
F3GetECPowerTable returns a F3 specific power table for use in standalone F3 nodes.


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "ID": 1000,
    "Power": 0,
    "PubKey": "Bw=="
  }
]
```

### F3GetF3PowerTable
F3GetF3PowerTable returns a F3 specific power table.


Perms: read

Inputs:
```json
[
  [
    {
      "/": "bafy2bzacea3wsdh6y3a36tb3skempjoxqpuyompjbmfeyf34fi3uy6uue42v4"
    },
    {
      "/": "bafy2bzacebp3shtrn43k7g3unredz7fxn4gj533d3o43tqn2p2ipxxhrvchve"
    }
  ]
]
```

Response:
```json
[
  {
    "ID": 1000,
    "Power": 0,
    "PubKey": "Bw=="
  }
]
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
*********************************** ALL F3 APIs below are not stable & subject to change ***********************************


Perms: sign

Inputs:
```json
[
  "f01234",
  "0001-01-01T00:00:00Z",
  "0001-01-01T00:00:00Z"
]
```

Response: `true`

