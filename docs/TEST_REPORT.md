# Test report — Artisan-Centred DPP chaincode prototype

**Date:** 2026-11-23  
**Local environment:** Go 1.23.2, Linux; no Docker/`peer` binary; dependency
network access blocked.  
**Executed against:** real `dpp.Contract` business methods with a test
implementation of Fabric-compatible state and caller interfaces.

## Results

| Test | Observed outcome |
|---|---|
| Successful multi-artisan ceramic passport and independent attestation | PASS |
| Supplier-to-workshop custody transfer | PASS |
| Material derivation and recursive multi-tier lot provenance | PASS |
| Authorization, roles and cross-MSP product boundaries | PASS |
| No over-consumption or duplicate lot allocation | PASS |
| Duplicate IDs, unknown artisan, invalid hash and schema validation | PASS |
| Anonymized/pseudonymous artisan public projection | PASS |
| Publication preconditions and post-publication write rejection | PASS |
| Independent audit: correct MSP, one attestation/MSP, public lookup | PASS |
| Standard SHA-256 test fixture | PASS |
| `go test -race -cover ./dpp -count=1 -v` | PASS (8 Go test functions) |
| `go vet ./dpp ./cmd/demo` | PASS |
| Offline demo and sample passport JSON parsing | PASS |

**Statement coverage (core `dpp` package): 75.1%.** This figure covers the
transaction core, not the Fabric adapter.

## What was NOT executed

- Building/linking `cmd/chaincode` against the actual
  `fabric-contract-api-go` library (the external module was unavailable).
- `TestFabricContractAPIRegistration` (requires the Fabric module download).
- Real Fabric test-network chaincode lifecycle install/approve/commit.
- Signed proposal, endorsement, transaction ordering, MVCC, invalid-write
  rollback, MSP/CA certificate attribute extraction, cross-org networking.
- Security audit and performance benchmarks.

The repository contains `cmd/chaincode/main_test.go` for Fabric Contract API
registration validation and the README has the integration procedure for a
networked host. **Do not interpret unit-test results as network integration or
production deployment validation.**

Test stdout is preserved in `docs/test-results.txt`; coverage instrumentation
is preserved in `coverage.out`.
