# CraftPass Studio — Artisan-Centred Digital Product Passport

**Hyperledger Fabric chaincode + Go REST API + browser interface + IEEE research paper.**

CraftPass tracks **where materials came from**, **which activities created each product**, and **which artisans contributed to each stage**. It publishes privacy-aware product passports and records independent audit assertions. This is an **academic reference prototype**, not a production-ready, legally certified DPP.

## Quick start: run the demo (no Docker or external libraries)

Requires Go **1.23+**. No `npm install` and no Fabric daemon are required for the demo.

```bash
go run ./cmd/server
# Visit http://127.0.0.1:8080
```

Demo data includes `vase-001`, three artisans (including a pseudonymous contributor), two supplier material lots, three production stages and an independent attestation. Choose the correct role in the top-right of the web interface when registering materials, producing products or auditing activities.

```bash
# Optional: persist demo ledger state between server restarts
DEMO_STATE_FILE=./data/demo-state.json go run ./cmd/server

# Test code that does not require Hyperledger modules
./scripts/test.sh
# Test REST API and web server too:
go test -race ./dpp ./api ./backend ./cmd/demo ./cmd/server
```

The demo uses the **same domain contract methods** as Fabric. It mocks the ledger/state interface, does not provide consensus, signatures, distributed identities or blockchain immutability. Demo actors can be switched in the UI; this is for local development only.

## Components

```text
cmd/server/web/       Zero-dependency HTML/CSS/JavaScript user interface
cmd/server/           Go HTTP server entrypoint; embed assets; build flags
api/                  REST endpoints + HTTP test suite
backend/demo.go       Local simulated ledger and persisted snapshot option
backend/fabric.go     Fabric Gateway adapter, gated by -tags fabric
cmd/chaincode/        Hyperledger Fabric Contract API adapter
cmd/demo/             CLI-generated passport example
dpp/                  Deterministic business logic and comprehensive tests
examples/             JSON transaction examples and Fabric profile template
docs/                 Architecture, REST API, testing, Fabric integration plan
paper/                IEEE paper LaTeX source and BibTeX references
.github/workflows/    Automated offline-Go verification
```

## Supported operations

| Operation | API endpoint | Domain transaction |
|---|---|---|
| Create artisan | `POST /api/artisans` | `RegisterArtisan` |
| Create material lot | `POST /api/materials` | `RegisterMaterial` |
| Change custodian | `POST /api/materials/{id}/transfer` | `TransferMaterial` |
| Derive material lot | `POST /api/materials/derive` | `DeriveMaterial` |
| Create product | `POST /api/products` | `CreateProduct` |
| Record production activity | `POST /api/activities` | `RecordActivity` |
| Publish passport | `POST /api/products/{id}/publish` | `PublishProduct` |
| Add auditor assertion | `POST /api/attestations` | `AttestActivity` |
| Read public passport | `GET /api/passports/{id}` | `GetPassport` |
| Individual ledger records | `GET /api/{artisans,materials,products,activities}/{id}` | `Get*` |
| Verify recorded evidence digest | `GET /api/activities/{id}/evidence/{sha256}` | `VerifyEvidence` |
| Demo-only catalogs | `GET /api/records/{kind}` | memory-index lookup |

Read **[docs/API.md](docs/API.md)** for curl examples, JSON payloads, errors, HTTP authentication and access restrictions.

## Fabric deployment

To use a **real** Fabric network, deploy the chaincode through Fabric's lifecycle tooling, configure server-side enrolled identities (with `dpp.role` certificate attributes), and compile the API with the Gateway tag. Refer to [docs/FABRIC_INTEGRATION_TEST_PLAN.md](docs/FABRIC_INTEGRATION_TEST_PLAN.md).

```bash
# In a networked environment with Fabric Gateway SDK, gRPC, and
# fabric-contract-api-go downloaded:
go mod tidy
go test ./cmd/chaincode -v
go build -o build/chaincode ./cmd/chaincode

go build -tags fabric -o build/server ./cmd/server
API_TOKEN='create-a-random-token-at-least-32-chars' \
FABRIC_CONFIG=./private/fabric-config.json \
DPP_BACKEND=fabric ./build/server
```

`examples/fabric-config.example.json` describes the server-side Fabric credentials **without including any secrets**. These credential files and Fabric private keys **must never be committed**. Identity selection in the web UI switches among **server-configured, enrolled Fabric identities**: an `X-Actor` header does not create or change a Fabric certificate. Nevertheless, a *single* `API_TOKEN` authorizes selection among all configured identities. This gateway design is suitable only for a **trusted research administration environment**. Before offering this to separate companies or individual users, implement per-user authentication/authorization and hard binding of API users to allowed Fabric identities, TLS and appropriate consent governance. **Do not expose the demo without restrictions to the internet.**

The current chaincode supports direct `Get*` reads by ID but **no enumerating queries**, so dashboard counts and record lists are available **only in demo mode**. In Fabric mode, query exact IDs.

## Example: read a public passport

```bash
curl http://127.0.0.1:8080/api/passports/vase-001
```

Then use the **Passports** section in the web UI to browse the production timeline, contributions, original materials, and attestations.

## Privacy and provenance boundaries

- Do not store personally identifying data on a shared Fabric channel. Even when a public JSON passport omits non-public aliases, underlying blockchain peers can inspect ledger data and history.
- `publicAlias` is a consented display alias, not verified legal identity. There is **no consent/withdrawal workflow** in this prototype.
- SHA-256 digest equality verifies content against a recorded hash but does **not** prove real-world provenance or authorship.
- Audit `supported` and `disputed` flags are **recorded assertions**, not independent cryptographic proof.
- Evidence documents, certificates and sensitive data should be maintained outside the chain with appropriate retention, access controls and signatures.
- HTTP API uses one bearer token (or no token in local demo) and must not be treated as production multi-tenant access control.
- Publish permanently prevents appending further activities under the current contract. Amendments require a designed event/versioning mechanism.

## Test status

**Executed locally:** core domain unit tests, race detector, HTTP integration tests, Go vet, JavaScript syntax validation, embedded static web serving, and demo API smoke tests.

**Not executed:** Fabric peer/network deployment, CA enrollment, multi-organization endorsement, MVCC, Fabric Gateway SDK compilation against downloaded dependencies, cryptographic identity verification and end-to-end distributed network tests. See [docs/TEST_REPORT.md](docs/TEST_REPORT.md) for the earlier domain results and [docs/API_TEST_REPORT.md](docs/API_TEST_REPORT.md) for API/web tests.

MIT licensed. See [LICENSE](LICENSE).
