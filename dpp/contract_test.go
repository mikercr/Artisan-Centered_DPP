package dpp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

type ledger struct {
	state  map[string][]byte
	events []string
	serial int
}

type transaction struct {
	l        *ledger
	identity Caller
	txid     string
}

func newLedger() *ledger { return &ledger{state: map[string][]byte{}} }
func (l *ledger) tx(ms, role string) *transaction {
	l.serial++
	return &transaction{l: l, txid: fmt.Sprintf("tx-%04d", l.serial), identity: Caller{MSPID: ms, Role: role, IDHash: "hashed-client-id"}}
}
func (t *transaction) GetState(k string) ([]byte, error) { return t.l.state[k], nil }
func (t *transaction) PutState(k string, v []byte) error {
	t.l.state[k] = append([]byte(nil), v...)
	return nil
}
func (t *transaction) GetTxID() string { return t.txid }
func (t *transaction) EmitEvent(n string, payload []byte) error {
	t.l.events = append(t.l.events, n)
	return nil
}
func (t *transaction) Caller() (Caller, error) { return t.identity, nil }

const evidence = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const auditEvidence = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

func data(v interface{}) string { b, _ := json.Marshal(v); return string(b) }
func needOK(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
func needErr(t *testing.T, err error, contains string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), contains) {
		t.Fatalf("expected error containing %q; got %v", contains, err)
	}
}

func registerArtisan(t *testing.T, l *ledger, id, alias string, public bool) {
	t.Helper()
	_, err := (&Contract{}).RegisterArtisan(l.tx("WorkshopMSP", "registrar"), data(ArtisanInput{ID: id, PublicAlias: alias, Public: public, Skills: []string{"pottery"}}))
	needOK(t, err)
}
func registerLot(t *testing.T, l *ledger, ms, id string, grams int64) {
	t.Helper()
	_, err := (&Contract{}).RegisterMaterial(l.tx(ms, "registrar"), data(MaterialInput{ID: id, MaterialType: "clay", OriginRegion: "Alentejo", QuantityGrams: grams, EvidenceSHA256: evidence}))
	needOK(t, err)
}
func createProduct(t *testing.T, l *ledger, id string) {
	t.Helper()
	_, err := (&Contract{}).CreateProduct(l.tx("WorkshopMSP", "producer"), data(ProductInput{ID: id, Name: "Ceramic bowl", Category: "ceramics"}))
	needOK(t, err)
}
func recordActivity(t *testing.T, l *ledger, id, product string, contributors []Contribution, materials []MaterialUse) {
	t.Helper()
	_, err := (&Contract{}).RecordActivity(l.tx("WorkshopMSP", "producer"), data(ActivityInput{ID: id, ProductID: product, Stage: "shaping", Technique: "wheel throwing", Contributors: contributors, Materials: materials, EvidenceSHA256: evidence}))
	needOK(t, err)
}

func TestEndToEndArtisanPassportAndAuditor(t *testing.T) {
	c, l := &Contract{}, newLedger()
	registerArtisan(t, l, "artisan-01", "Maria (public alias)", true)
	registerArtisan(t, l, "artisan-02", "Joao (public alias)", true)
	registerArtisan(t, l, "artisan-03", "", false)
	registerLot(t, l, "SupplierMSP", "clay-01", 1500)
	_, err := c.TransferMaterial(l.tx("SupplierMSP", "registrar"), "clay-01", "WorkshopMSP")
	needOK(t, err)
	registerLot(t, l, "WorkshopMSP", "glaze-01", 250)
	createProduct(t, l, "vase-001")
	recordActivity(t, l, "shaping-01", "vase-001", []Contribution{{ArtisanID: "artisan-01", Role: "potter"}}, []MaterialUse{{LotID: "clay-01", QuantityGrams: 700}})
	recordActivity(t, l, "glazing-01", "vase-001", []Contribution{{ArtisanID: "artisan-02", Role: "decorator"}, {ArtisanID: "artisan-03", Role: "assistant"}}, []MaterialUse{{LotID: "glaze-01", QuantityGrams: 150}})
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "vase-001")
	needOK(t, err)
	att, err := c.AttestActivity(l.tx("CraftAuthorityMSP", "auditor"), data(AttestationInput{ActivityID: "shaping-01", Verdict: "supported", EvidenceSHA256: auditEvidence}))
	needOK(t, err)
	if att.AuditorMSP != "CraftAuthorityMSP" {
		t.Fatalf("wrong attestation %#v", att)
	}
	passport, err := c.GetPassport(l.tx("PublicMSP", "reader"), "vase-001")
	needOK(t, err)
	if passport.ID != "vase-001" || len(passport.Activities) != 2 || len(passport.Materials) != 2 {
		t.Fatalf("unexpected passport %#v", passport)
	}
	if passport.Activities[0].Artisans[0].Alias != "Maria (public alias)" {
		t.Fatal("public alias missing")
	}
	if passport.Activities[1].Artisans[1].Alias != "" {
		t.Fatal("private artisan alias exposed")
	}
	if len(passport.Activities[0].Attestations) != 1 || passport.Activities[0].Attestations[0].Verdict != "supported" {
		t.Fatalf("attestations absent %#v", passport.Activities[0])
	}
	clay, err := c.GetMaterial(l.tx("PublicMSP", "reader"), "clay-01")
	needOK(t, err)
	if clay.UsedGrams != 700 || len(clay.Transfers) != 1 || clay.Transfers[0].FromMSP != "SupplierMSP" {
		t.Fatalf("incorrect custody/usage %#v", clay)
	}
	match, err := c.VerifyEvidence(l.tx("PublicMSP", "reader"), "shaping-01", evidence)
	needOK(t, err)
	if !match {
		t.Fatal("matching evidence hash was rejected")
	}
	match, err = c.VerifyEvidence(l.tx("PublicMSP", "reader"), "shaping-01", auditEvidence)
	needOK(t, err)
	if match {
		t.Fatal("incorrect evidence accepted")
	}
	if len(l.events) != 11 {
		t.Fatalf("expected 11 emitted events; got %d: %#v", len(l.events), l.events)
	}
}

func TestMaterialLineageAndConservation(t *testing.T) {
	c, l := &Contract{}, newLedger()
	registerArtisan(t, l, "a", "A", true)
	registerLot(t, l, "WorkshopMSP", "clay", 1000)
	registerLot(t, l, "WorkshopMSP", "grog", 200)
	derived, err := c.DeriveMaterial(l.tx("WorkshopMSP", "producer"), data(DerivedLotInput{ID: "mix", MaterialType: "clay blend", QuantityGrams: 540, SourceLots: []MaterialUse{{LotID: "clay", QuantityGrams: 400}, {LotID: "grog", QuantityGrams: 200}}}))
	needOK(t, err)
	if derived.OriginRegion != "mixed" || len(derived.SourceLots) != 2 {
		t.Fatal("missing derived ancestry")
	}
	createProduct(t, l, "bowl")
	recordActivity(t, l, "mixing", "bowl", []Contribution{{ArtisanID: "a", Role: "potter"}}, []MaterialUse{{LotID: "mix", QuantityGrams: 500}})
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "bowl")
	needOK(t, err)
	passport, err := c.GetPassport(l.tx("public", "reader"), "bowl")
	needOK(t, err)
	if len(passport.Materials) != 3 || passport.Materials[0].ID != "clay" || passport.Materials[1].ID != "grog" || passport.Materials[2].ID != "mix" {
		t.Fatalf("incorrect lineage: %+v", passport.Materials)
	}
	clay, err := c.GetMaterial(l.tx("public", "reader"), "clay")
	needOK(t, err)
	if clay.UsedGrams != 400 {
		t.Fatalf("expected source 400 g consumed; got %d", clay.UsedGrams)
	}
	mix, err := c.GetMaterial(l.tx("public", "reader"), "mix")
	needOK(t, err)
	if mix.UsedGrams != 500 {
		t.Fatalf("expected derived 500 g consumed; got %d", mix.UsedGrams)
	}
}

func TestAuthorizationAndIsolation(t *testing.T) {
	c, l := &Contract{}, newLedger()
	_, err := c.RegisterArtisan(l.tx("WorkshopMSP", "reader"), data(ArtisanInput{ID: "a", Public: true, PublicAlias: "A"}))
	needErr(t, err, "access denied")
	_, err = c.CreateProduct(l.tx("WorkshopMSP", "registrar"), data(ProductInput{ID: "p", Name: "P", Category: "craft"}))
	needErr(t, err, "access denied")
	registerArtisan(t, l, "a", "A", true)
	registerLot(t, l, "SupplierMSP", "lot", 100)
	createProduct(t, l, "p")
	_, err = c.RecordActivity(l.tx("OtherMSP", "producer"), data(ActivityInput{ID: "act", ProductID: "p", Stage: "shaping", Technique: "manual", Contributors: []Contribution{{ArtisanID: "a", Role: "maker"}}}))
	needErr(t, err, "another organization's product")
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(ActivityInput{ID: "act", ProductID: "p", Stage: "shaping", Technique: "manual", Contributors: []Contribution{{ArtisanID: "a", Role: "maker"}}, Materials: []MaterialUse{{LotID: "lot", QuantityGrams: 10}}}))
	needErr(t, err, "not in product owner's custody")
	_, err = c.TransferMaterial(l.tx("OtherMSP", "producer"), "lot", "WorkshopMSP")
	needErr(t, err, "only current")
	_, err = c.TransferMaterial(l.tx("SupplierMSP", "registrar"), "lot", "WorkshopMSP")
	needOK(t, err)
	_, err = c.AttestActivity(l.tx("OtherMSP", "producer"), data(AttestationInput{ActivityID: "act", Verdict: "supported", EvidenceSHA256: auditEvidence}))
	needErr(t, err, "access denied")
}

func TestRejectionPreservesBalances(t *testing.T) {
	c, l := &Contract{}, newLedger()
	registerArtisan(t, l, "a", "A", true)
	registerLot(t, l, "WorkshopMSP", "lot", 100)
	createProduct(t, l, "p")
	base := ActivityInput{ID: "act", ProductID: "p", Stage: "work", Technique: "manual", Contributors: []Contribution{{ArtisanID: "a", Role: "maker"}}}
	tooMuch := base
	tooMuch.Materials = []MaterialUse{{LotID: "lot", QuantityGrams: 101}}
	_, err := c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(tooMuch))
	needErr(t, err, "insufficient")
	lot, err := c.GetMaterial(l.tx("reader", "reader"), "lot")
	needOK(t, err)
	if lot.UsedGrams != 0 {
		t.Fatal("failed validation must not consume material")
	}
	duplicate := base
	duplicate.Materials = []MaterialUse{{LotID: "lot", QuantityGrams: 10}, {LotID: "lot", QuantityGrams: 5}}
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(duplicate))
	needErr(t, err, "duplicate material lot")
	base.Materials = []MaterialUse{{LotID: "lot", QuantityGrams: 80}}
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(base))
	needOK(t, err)
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(base))
	needErr(t, err, "already exists")
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needOK(t, err)
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(ActivityInput{ID: "next", ProductID: "p", Stage: "glaze", Technique: "manual", Contributors: base.Contributors}))
	needErr(t, err, "published")
	lot, err = c.GetMaterial(l.tx("reader", "reader"), "lot")
	needOK(t, err)
	if lot.UsedGrams != 80 {
		t.Fatalf("expected used=80, got %d", lot.UsedGrams)
	}
}

func TestInputValidationAndPrivacy(t *testing.T) {
	c, l := &Contract{}, newLedger()
	_, err := c.RegisterArtisan(l.tx("WorkshopMSP", "registrar"), `{"id":"private","public":false,"publicAlias":"Secret"}`)
	needErr(t, err, "private artisan")
	_, err = c.RegisterArtisan(l.tx("WorkshopMSP", "registrar"), `{"id":"x","public":true,"publicAlias":"A","unknown":1}`)
	needErr(t, err, "unknown field")
	_, err = c.RegisterMaterial(l.tx("WorkshopMSP", "registrar"), data(MaterialInput{ID: "lot", MaterialType: "clay", OriginRegion: "PT", QuantityGrams: 100, EvidenceSHA256: "abc"}))
	needErr(t, err, "evidenceSha256")
	registerArtisan(t, l, "a", "A", true)
	registerLot(t, l, "WorkshopMSP", "lot", 100)
	createProduct(t, l, "p")
	_, err = c.GetPassport(l.tx("any", "reader"), "p")
	needErr(t, err, "unavailable")
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needErr(t, err, "needs at least one")
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(ActivityInput{ID: "bad", ProductID: "p", Stage: "shaping", Technique: "manual", Contributors: []Contribution{{ArtisanID: "ghost", Role: "potter"}}}))
	needErr(t, err, "artisan \"ghost\" not found")
	_, err = c.DeriveMaterial(l.tx("WorkshopMSP", "producer"), data(DerivedLotInput{ID: "bad-mix", MaterialType: "blend", QuantityGrams: 101, SourceLots: []MaterialUse{{LotID: "lot", QuantityGrams: 100}}}))
	needErr(t, err, "cannot exceed input")
	_, err = c.RecordActivity(l.tx("WorkshopMSP", "producer"), data(ActivityInput{ID: "act", ProductID: "p", Stage: "shaping", Technique: "manual", Contributors: []Contribution{{ArtisanID: "a", Role: "potter"}}, Materials: []MaterialUse{{LotID: "lot", QuantityGrams: 10}}}))
	needOK(t, err)
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needOK(t, err)
	_, err = c.AttestActivity(l.tx("WorkshopMSP", "auditor"), data(AttestationInput{ActivityID: "act", Verdict: "supported", EvidenceSHA256: auditEvidence}))
	needErr(t, err, "own MSP")
	_, err = c.AttestActivity(l.tx("AuditorMSP", "auditor"), data(AttestationInput{ActivityID: "act", Verdict: "supported", EvidenceSHA256: auditEvidence}))
	needOK(t, err)
	_, err = c.AttestActivity(l.tx("AuditorMSP", "auditor"), data(AttestationInput{ActivityID: "act", Verdict: "disputed", EvidenceSHA256: evidence}))
	needErr(t, err, "already attested")
}

func TestDeterministicHashExample(t *testing.T) {
	h := sha256.Sum256([]byte("signed-off-chain-report"))
	if got, want := hex.EncodeToString(h[:]), "2eb882ee0e80fb88003292672ecddbc2da0f074cf65ca0186f27f0a79033ddb6"; got != want {
		t.Fatalf("wrong digest: got %s want %s", got, want)
	}
}

func TestPublishingRequiresMaterialAndAttestationQueries(t *testing.T) {
	c, l := &Contract{}, newLedger()
	registerArtisan(t, l, "a", "A", true)
	createProduct(t, l, "p")
	recordActivity(t, l, "assembly", "p", []Contribution{{ArtisanID: "a", Role: "maker"}}, nil)
	_, err := c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needErr(t, err, "at least one material")
	registerLot(t, l, "WorkshopMSP", "lot", 100)
	recordActivity(t, l, "material-use", "p", []Contribution{{ArtisanID: "a", Role: "maker"}}, []MaterialUse{{LotID: "lot", QuantityGrams: 20}})
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needOK(t, err)
	_, err = c.AttestActivity(l.tx("AuditorMSP", "auditor"), data(AttestationInput{ActivityID: "material-use", Verdict: "disputed", EvidenceSHA256: auditEvidence}))
	needOK(t, err)
	a, err := c.GetAttestation(l.tx("AuditorMSP", "reader"), "material-use", "AuditorMSP")
	needOK(t, err)
	if a.Verdict != "disputed" {
		t.Fatal("missing disputed attestation")
	}
	passport, err := c.GetPassport(l.tx("WorkshopMSP", "reader"), "p")
	needOK(t, err)
	if len(passport.Activities) != 2 || len(passport.Activities[1].Attestations) != 1 {
		t.Fatal("missing attestation in passport")
	}
	_, err = c.GetAttestation(l.tx("any", "reader"), "assembly", "UnknownMSP")
	needErr(t, err, "not found")
	_, err = c.PublishProduct(l.tx("WorkshopMSP", "producer"), "p")
	needErr(t, err, "already published")
}

func TestValidationOfBadIDsAndTransfers(t *testing.T) {
	c, l := &Contract{}, newLedger()
	_, err := c.RegisterMaterial(l.tx("WorkshopMSP", "registrar"), data(MaterialInput{ID: "bad/id", MaterialType: "clay", OriginRegion: "PT", QuantityGrams: 10}))
	needErr(t, err, "invalid id")
	registerLot(t, l, "WorkshopMSP", "lot", 90)
	_, err = c.TransferMaterial(l.tx("WorkshopMSP", "producer"), "lot", "WorkshopMSP")
	needErr(t, err, "same MSP")
	_, err = c.RegisterMaterial(l.tx("WorkshopMSP", "registrar"), data(MaterialInput{ID: "lot", MaterialType: "clay", OriginRegion: "PT", QuantityGrams: 90}))
	needErr(t, err, "already exists")
	_, err = c.DeriveMaterial(l.tx("WorkshopMSP", "producer"), data(DerivedLotInput{ID: "blend", MaterialType: "blend", QuantityGrams: 10, SourceLots: []MaterialUse{{LotID: "lot", QuantityGrams: 10}, {LotID: "lot", QuantityGrams: 15}}}))
	needErr(t, err, "duplicate material lot")
	_, err = c.DeriveMaterial(l.tx("WorkshopMSP", "producer"), data(DerivedLotInput{ID: "blend", MaterialType: "blend", QuantityGrams: 10, SourceLots: []MaterialUse{{LotID: "lot", QuantityGrams: 91}}}))
	needErr(t, err, "insufficient")
	_, err = c.GetArtisan(l.tx("any", "reader"), "no-such-artisan")
	needErr(t, err, "not found")
}
