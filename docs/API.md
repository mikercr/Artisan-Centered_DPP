# REST API: CraftPass Studio

Base URL: `http://127.0.0.1:8080`. The standalone server serves the browser UI at `/` and static assets under `/assets/*`.

All JSON API replies have the form `{"data": ...}` on success or `{"error":"message"}` on errors, except `/api/meta` and `/api/health` which return top-level metadata objects.

## Authentication and identity

In demo mode, authentication is optional unless `API_TOKEN` is set. Add these headers when a token is configured:

```http
Authorization: Bearer <token>
X-Actor: workshop-producer
Content-Type: application/json
```

For Fabric deployments, the backend selects a **preconfigured server-side enrolled credential** matching `X-Actor`. There is no client-provided certificate enrollment, and the actor's actual chaincode role comes from the Fabric certificate attribute `dpp.role`. The single bearer token is **not multi-user authorization**; all authorized callers can select any server-configured actor. Run only behind a trusted admin gateway until per-user authorization and identity mapping are implemented. Public passport queries do not require a bearer token.

Demo actor IDs: `workshop-registrar`, `workshop-producer`, `supplier-registrar`, `independent-auditor`, `public-reader`.

## Operations

| HTTP | Path | Contract method | Required Fabric role |
|---|---|---|---|
| `GET` | `/api/health` | N/A | Public |
| `GET` | `/api/meta` | N/A | Public |
| `POST` | `/api/artisans` | `RegisterArtisan` | registrar |
| `POST` | `/api/materials` | `RegisterMaterial` | registrar |
| `POST` | `/api/materials/{id}/transfer` | `TransferMaterial` | custodian registrar/producer |
| `POST` | `/api/materials/derive` | `DeriveMaterial` | custodian registrar/producer |
| `POST` | `/api/products` | `CreateProduct` | producer |
| `POST` | `/api/activities` | `RecordActivity` | producer |
| `POST` | `/api/products/{id}/publish` | `PublishProduct` | product owner producer |
| `POST` | `/api/attestations` | `AttestActivity` | independent auditor |
| `GET` | `/api/passports/{id}` | `GetPassport` | Public |
| `GET` | `/api/artisans/{id}` | `GetArtisan` | Authenticated |
| `GET` | `/api/materials/{id}` | `GetMaterial` | Authenticated |
| `GET` | `/api/products/{id}` | `GetProduct` | Authenticated |
| `GET` | `/api/activities/{id}` | `GetActivity` | Authenticated |
| `GET` | `/api/activities/{id}/attestations/{msp}` | `GetAttestation` | Authenticated |
| `GET` | `/api/activities/{id}/evidence/{sha256}` | `VerifyEvidence` | Authenticated |
| `GET` | `/api/records/{kind}` | Demo-only catalog | Authenticated |

Body limit: **64 KiB**; POST payloads must be JSON objects.

## Curl examples

### Register an artisan

```bash
curl -X POST localhost:8080/api/artisans -H 'Content-Type: application/json' \
  -H 'X-Actor: workshop-registrar' \
  -d '{"id":"artisan-julia","publicAlias":"Júlia","public":true,"skills":["Wheel throwing"]}'
```

### Register a material lot at the supplier

```bash
curl -X POST localhost:8080/api/materials -H 'Content-Type: application/json' \
 -H 'X-Actor: supplier-registrar' \
 -d '{"id":"clay-2026-2","materialType":"Clay","originRegion":"Portugal","quantityGrams":3000}'
```

### Transfer custody to the workshop

```bash
curl -X POST localhost:8080/api/materials/clay-2026-2/transfer \
  -H 'Content-Type: application/json' -H 'X-Actor: supplier-registrar' \
  -d '{"toMSP":"WorkshopMSP"}'
```

### Derive a lot from one or more input lots

```bash
curl -X POST localhost:8080/api/materials/derive -H 'Content-Type: application/json' \
  -H 'X-Actor: workshop-producer' \
  -d '{"id":"clay-blend-2","materialType":"Clay blend","quantityGrams":1500,"sourceLots":[{"lotId":"clay-2026-2","quantityGrams":1600}]}'
```

### Register a product and its production activity

```bash
curl -X POST localhost:8080/api/products -H 'Content-Type: application/json' \
  -H 'X-Actor: workshop-producer' \
  -d '{"id":"bowl-2","name":"Traditional ceramic bowl","category":"Ceramics"}'

curl -X POST localhost:8080/api/activities -H 'Content-Type: application/json' \
  -H 'X-Actor: workshop-producer' \
  -d '{"id":"bowl-2-shaping","productId":"bowl-2","stage":"Shaping","technique":"Wheel throwing","contributors":[{"artisanId":"artisan-julia","role":"Potter"}],"materials":[{"lotId":"clay-blend-2","quantityGrams":850}]}'
```

### Publish and read the DPP

```bash
curl -X POST localhost:8080/api/products/bowl-2/publish \
 -H 'X-Actor: workshop-producer'
curl localhost:8080/api/passports/bowl-2
```

### Submit an independent auditor attestation

```bash
curl -X POST localhost:8080/api/attestations -H 'Content-Type: application/json' \
 -H 'X-Actor: independent-auditor' \
 -d '{"activityId":"bowl-2-shaping","verdict":"supported","evidenceSha256":"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}'
```

### Error handling

API maps validation failures to `400`, missing objects to `404`, conflicts to `409`, authorization failures to `403`, absent/incorrect bearer token to `401`, Fabric connectivity failures to `502`, and unsupported catalog enumeration in Fabric to `501`. HTTP errors should be handled explicitly by clients; do not infer blockchain success from a request timeout.

## Limitations

The demo-state JSON file is **not a secure ledger**, the read API deliberately exposes detailed records only behind the shared API token in Fabric mode, and a public DPP is a **projection** (non-public artisan aliases are omitted). Fabric peers can still inspect raw ledger data. This must not be deployed with personal details on a broadly shared channel.
