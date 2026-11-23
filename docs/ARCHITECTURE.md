# Data design, semantics and trust boundaries

## Projection of the article's semantic model

| Paper concept | Fabric JSON asset / field | Ontological interpretation |
|---|---|---|
| `CraftProduct` | `Product` | `prov:Entity`, cultural object |
| `Material` | `MaterialLot` | `prov:Entity`, `prov:wasDerivedFrom` for parent lots |
| `CraftActivity` | `Activity` | `prov:Activity`, `prov:used` lots |
| `Artisan` | `Artisan` | `prov:Agent` |
| `Contribution` | `Activity.contributors[]` | `prov:qualifiedAssociation`, `prov:agent`, `prov:hadRole` |
| `Technique` | `Activity.technique` | Domain-controlled term, not a published ontology URI yet |
| `Location` | `Activity.locationCode` | Coded location, not a publicly revealing address |
| `Evidence` | `evidenceSha256` | Hash of an off-chain claim/document |
| `Attestation` | `Attestation` | Independent auditor assertion, not truth proof |

This prototype deliberately **does not** claim RDF/OWL, PROV-O or CIDOC CRM
native storage. The above is a bridge towards a later JSON-LD mapping, SHACL
validation, ontology reuse, EPCIS event integration, and SPARQL queries.

## Provenance path

- Each product points to an ordered list of `ActivityIDs`.
- Each activity associates **one or more** artisans and roles with a technique.
- Each activity consumes **zero or more** material-lot allocations.
- A derived lot contains its parent-lot references and consumed quantities.
- `GetPassport` walks these relationships recursively and sorts the final
  material projection by ID for deterministic presentation.
- Auditor assertions are indexed from the corresponding activity by MSP ID.

## Trust model

Fabric provides authenticated organizations, an append-only transaction log,
endorsement policies, and MVCC validation. The contract enforces application
roles via the certificate attribute `dpp.role` and checks material/product
custody by MSP, but **the choice of MSP membership/endorsement policy is a
network administrator task**.

The contract *does not* claim blockchain itself proves the accuracy of a
reported artisan's contribution, geographic origin, or certificate. Hashes
make later off-chain comparison possible; independent verifications are
recorded as attestations with the auditor's MSP.

The `GetPassport` response is intended for a vetted API gateway. Channel
participants must not be assumed to be consumers. A consumer-facing QR
landing page would normally fetch a policy-filtered view from that gateway.
The chaincode's read methods have no general public HTTP exposure.
