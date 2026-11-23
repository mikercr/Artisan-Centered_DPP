# Network integration acceptance tests (NOT executed in this environment)

This checklist is intended for the Hyperledger Fabric `fabric-samples/test-network`
with Fabric CA and a running orderer. At minimum, use two MSPs:

- `Org1MSP` — the ceramic workshop (roles `registrar`, `producer`).
- `Org2MSP` — the clay supplier (`registrar`) and an *independent* auditor
  (`auditor`, a separate client certificate).

Each role must be embedded in its enrollment certificate as `dpp.role`.
For write transaction tests, the caller's authenticated MSP and role must
match the action. Check the Fabric CA version's registration syntax, including
`dpp.role=...:ecert` for attributes exposed in enrollment certificates.

1. Start the Fabric network and create `mychannel` with the Fabric CA enabled.
2. Deploy `acdpp` using `./network.sh deployCC -ccn acdpp -ccp <repo> -ccl go`.
3. With Org1 registrar, register `artisan-04` as public and `artisan-05`
   without public alias. Confirm both IDs are returned.
4. With Org2 registrar, register `clay-batch-08` (3,000 grams). Transfer it to
   `Org1MSP`. Confirm `GetMaterial` reports the transfer and current custodian.
5. With Org1 producer, create `vase-023`, record shaping using 800 grams of clay
   with artisan-04, and firing involving artisan-05. Publish.
6. Query `GetPassport("vase-023")`; verify two activities, their distinct
   artisan roles, public alias only for artisan-04, source lot and origin.
7. With Org2 auditor, attest shaping as `supported`; query again and verify the
   attestation's MSP, verdict and evidence hash.
8. Negative: Org1 producer attempts `RegisterMaterial` and receives role denial.
9. Negative: an Org2 producer attempts to update Org1's product and is denied.
10. Negative: attempt to consume more than remaining material and verify it is
    rejected *and* remaining balance is unchanged after block processing.
11. Negative: an Org1 auditor tries to attest Org1's activity and is denied.
12. Negative: duplicate activity ID / duplicate auditor MSP rejected.
13. Verify each submitted write's commit status using Fabric Gateway; do not
    rely only on successful endorsement. Monitor chaincode event delivery and
    check an intentional write conflict for MVCC validation behavior.
14. Run concurrent writes to the same lot and confirm endorsed transactions do
    not jointly overconsume material (at most the valid committed write wins
    when simulations share a stale state version).
15. Run metadata/reflection validation: `go test ./cmd/chaincode -v`.
16. Check that channel peers still have ordinary ledger access; add a private
    collection in a subsequent security iteration if restricted data is needed.

Report actual Fabric version, chaincode version, number of MSPs/peers,
endorsement policy, transaction IDs, commit statuses, and execution environment.
Do not report throughput/latency results until measured on that network.
