package dpp

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,63}$`)
var digestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func decode(input string, target interface{}) error {
	if len(input) > 65536 {
		return errors.New("payload exceeds 64 KiB")
	}
	d := json.NewDecoder(strings.NewReader(input))
	d.DisallowUnknownFields()
	if err := d.Decode(target); err != nil {
		return fmt.Errorf("invalid JSON: %w", err)
	}
	var extra interface{}
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return errors.New("multiple JSON values are not allowed")
	}
	return nil
}

func validateID(id string) error {
	if !idPattern.MatchString(id) {
		return fmt.Errorf("invalid id %q (1-64 alphanumeric/._:- characters)", id)
	}
	return nil
}

func nonEmpty(s, name string) error {
	if strings.TrimSpace(s) == "" || len(s) > 160 {
		return fmt.Errorf("%s must contain 1-160 characters", name)
	}
	return nil
}

func digestIfPresent(s string) error {
	if s != "" && !digestPattern.MatchString(s) {
		return errors.New("evidenceSha256 must be 64 lowercase hexadecimal characters")
	}
	return nil
}

func require(ctx TxContext, roles ...string) (Caller, error) {
	caller, err := ctx.Caller()
	if err != nil {
		return Caller{}, err
	}
	if caller.MSPID == "" || caller.IDHash == "" {
		return Caller{}, errors.New("missing authenticated Fabric identity")
	}
	for _, r := range roles {
		if caller.Role == r {
			return caller, nil
		}
	}
	return Caller{}, fmt.Errorf("access denied: requires one of roles %v", roles)
}

func key(kind, id string) string { return kind + "~" + id }

func load[T any](ctx TxContext, kind, id string) (*T, error) {
	if err := validateID(id); err != nil {
		return nil, err
	}
	v, err := ctx.GetState(key(kind, id))
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("%s %q not found", kind, id)
	}
	var doc T
	if err := json.Unmarshal(v, &doc); err != nil {
		return nil, fmt.Errorf("corrupt %s %q: %w", kind, id, err)
	}
	return &doc, nil
}

func unused(ctx TxContext, kind, id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	v, err := ctx.GetState(key(kind, id))
	if err != nil {
		return err
	}
	if v != nil {
		return fmt.Errorf("%s %q already exists", kind, id)
	}
	return nil
}

func put(ctx TxContext, kind, id string, doc interface{}) error {
	payload, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return ctx.PutState(key(kind, id), payload)
}

func event(ctx TxContext, name string, id string) error {
	payload, _ := json.Marshal(map[string]string{"id": id, "txId": ctx.GetTxID()})
	return ctx.EmitEvent(name, payload)
}

func uniqueInputs(inputs []MaterialUse) error {
	seen := make(map[string]bool)
	for _, in := range inputs {
		if err := validateID(in.LotID); err != nil {
			return err
		}
		if in.QuantityGrams <= 0 {
			return errors.New("quantityGrams must be positive")
		}
		if seen[in.LotID] {
			return fmt.Errorf("duplicate material lot %q", in.LotID)
		}
		seen[in.LotID] = true
	}
	return nil
}

// RegisterArtisan registers a pseudonymous artisan. If public=false, any
// submitted alias MUST be omitted: personal data does not belong on-chain.
func (c *Contract) RegisterArtisan(ctx TxContext, payload string) (*Artisan, error) {
	caller, err := require(ctx, "registrar")
	if err != nil {
		return nil, err
	}
	var in ArtisanInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	if err := unused(ctx, "artisan", in.ID); err != nil {
		return nil, err
	}
	if !in.Public && in.PublicAlias != "" {
		return nil, errors.New("private artisan must not include publicAlias")
	}
	if in.Public {
		if err := nonEmpty(in.PublicAlias, "publicAlias"); err != nil {
			return nil, err
		}
	}
	for _, skill := range in.Skills {
		if err := nonEmpty(skill, "skill"); err != nil {
			return nil, err
		}
	}
	if len(in.Skills) > 30 {
		return nil, errors.New("too many skills")
	}
	a := &Artisan{ID: in.ID, PublicAlias: in.PublicAlias, Public: in.Public, Skills: in.Skills, RegisteredByMSP: caller.MSPID, CreatedTxID: ctx.GetTxID()}
	if err := put(ctx, "artisan", in.ID, a); err != nil {
		return nil, err
	}
	return a, event(ctx, "ArtisanRegistered", in.ID)
}

// RegisterMaterial registers an untransformed source lot in grams.
func (c *Contract) RegisterMaterial(ctx TxContext, payload string) (*MaterialLot, error) {
	caller, err := require(ctx, "registrar")
	if err != nil {
		return nil, err
	}
	var in MaterialInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	if err := unused(ctx, "lot", in.ID); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.MaterialType, "materialType"); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.OriginRegion, "originRegion"); err != nil {
		return nil, err
	}
	if in.QuantityGrams <= 0 {
		return nil, errors.New("quantityGrams must be positive")
	}
	if err := digestIfPresent(in.EvidenceSHA256); err != nil {
		return nil, err
	}
	if len(in.SupplierRef) > 160 {
		return nil, errors.New("supplierRef too long")
	}
	lot := &MaterialLot{ID: in.ID, MaterialType: in.MaterialType, OriginRegion: in.OriginRegion, SupplierRef: in.SupplierRef, QuantityGrams: in.QuantityGrams, EvidenceSHA256: in.EvidenceSHA256, CreatedByMSP: caller.MSPID, CustodianMSP: caller.MSPID, CreatedTxID: ctx.GetTxID()}
	if err := put(ctx, "lot", in.ID, lot); err != nil {
		return nil, err
	}
	return lot, event(ctx, "MaterialRegistered", in.ID)
}

// TransferMaterial changes custody of the remaining quantity; the original
// source and complete custody sequence remain in the lot record.
func (c *Contract) TransferMaterial(ctx TxContext, lotID, toMSP string) (*MaterialLot, error) {
	caller, err := require(ctx, "registrar", "producer")
	if err != nil {
		return nil, err
	}
	if err := nonEmpty(toMSP, "toMSP"); err != nil {
		return nil, err
	}
	lot, err := load[MaterialLot](ctx, "lot", lotID)
	if err != nil {
		return nil, err
	}
	if lot.CustodianMSP != caller.MSPID {
		return nil, errors.New("only current material custodian can transfer")
	}
	if lot.CustodianMSP == toMSP {
		return nil, errors.New("cannot transfer to same MSP")
	}
	if lot.UsedGrams == lot.QuantityGrams {
		return nil, errors.New("material is fully consumed")
	}
	lot.Transfers = append(lot.Transfers, Transfer{FromMSP: caller.MSPID, ToMSP: toMSP, TxID: ctx.GetTxID()})
	lot.CustodianMSP = toMSP
	if err := put(ctx, "lot", lotID, lot); err != nil {
		return nil, err
	}
	return lot, event(ctx, "MaterialTransferred", lotID)
}

// DeriveMaterial combines or splits existing custodied lots, consuming their
// quantities and recording parent links. Output mass cannot exceed input mass.
func (c *Contract) DeriveMaterial(ctx TxContext, payload string) (*MaterialLot, error) {
	caller, err := require(ctx, "producer", "registrar")
	if err != nil {
		return nil, err
	}
	var in DerivedLotInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	if err := unused(ctx, "lot", in.ID); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.MaterialType, "materialType"); err != nil {
		return nil, err
	}
	if err := digestIfPresent(in.EvidenceSHA256); err != nil {
		return nil, err
	}
	if len(in.SourceLots) == 0 || len(in.SourceLots) > 50 {
		return nil, errors.New("sourceLots must contain 1-50 lots")
	}
	if err := uniqueInputs(in.SourceLots); err != nil {
		return nil, err
	}
	if in.QuantityGrams <= 0 {
		return nil, errors.New("quantityGrams must be positive")
	}
	var total int64
	origin := "mixed"
	parents := make([]*MaterialLot, 0, len(in.SourceLots))
	for _, src := range in.SourceLots {
		lot, err := load[MaterialLot](ctx, "lot", src.LotID)
		if err != nil {
			return nil, err
		}
		if lot.CustodianMSP != caller.MSPID {
			return nil, fmt.Errorf("cannot consume non-custodied material %q", src.LotID)
		}
		if src.QuantityGrams > lot.QuantityGrams-lot.UsedGrams {
			return nil, fmt.Errorf("insufficient material in lot %q", src.LotID)
		}
		if src.QuantityGrams > int64(^uint64(0)>>1)-total {
			return nil, errors.New("quantity overflow")
		}
		total += src.QuantityGrams
		parents = append(parents, lot)
	}
	if in.QuantityGrams > total {
		return nil, errors.New("derived output mass cannot exceed input mass")
	}
	if len(parents) == 1 {
		origin = parents[0].OriginRegion
	}
	derived := &MaterialLot{ID: in.ID, MaterialType: in.MaterialType, OriginRegion: origin, QuantityGrams: in.QuantityGrams, SourceLots: in.SourceLots, EvidenceSHA256: in.EvidenceSHA256, CreatedByMSP: caller.MSPID, CustodianMSP: caller.MSPID, CreatedTxID: ctx.GetTxID()}
	// The Fabric write set commits atomically, including all input-lot updates.
	for i, parent := range parents {
		parent.UsedGrams += in.SourceLots[i].QuantityGrams
		if err := put(ctx, "lot", parent.ID, parent); err != nil {
			return nil, err
		}
	}
	if err := put(ctx, "lot", in.ID, derived); err != nil {
		return nil, err
	}
	return derived, event(ctx, "MaterialDerived", in.ID)
}

func (c *Contract) CreateProduct(ctx TxContext, payload string) (*Product, error) {
	caller, err := require(ctx, "producer")
	if err != nil {
		return nil, err
	}
	var in ProductInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	if err := unused(ctx, "product", in.ID); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.Name, "name"); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.Category, "category"); err != nil {
		return nil, err
	}
	p := &Product{ID: in.ID, Name: in.Name, Category: in.Category, OwnerMSP: caller.MSPID, Status: "draft", ActivityIDs: []string{}, CreatedTxID: ctx.GetTxID()}
	if err := put(ctx, "product", in.ID, p); err != nil {
		return nil, err
	}
	return p, event(ctx, "ProductCreated", in.ID)
}

func (c *Contract) RecordActivity(ctx TxContext, payload string) (*Activity, error) {
	caller, err := require(ctx, "producer")
	if err != nil {
		return nil, err
	}
	var in ActivityInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	if err := unused(ctx, "activity", in.ID); err != nil {
		return nil, err
	}
	product, err := load[Product](ctx, "product", in.ProductID)
	if err != nil {
		return nil, err
	}
	if product.OwnerMSP != caller.MSPID {
		return nil, errors.New("cannot modify another organization's product")
	}
	if product.Status != "draft" {
		return nil, errors.New("cannot append activities to published product")
	}
	if err := nonEmpty(in.Stage, "stage"); err != nil {
		return nil, err
	}
	if err := nonEmpty(in.Technique, "technique"); err != nil {
		return nil, err
	}
	if len(in.LocationCode) > 160 {
		return nil, errors.New("locationCode too long")
	}
	if err := digestIfPresent(in.EvidenceSHA256); err != nil {
		return nil, err
	}
	if len(in.Contributors) == 0 || len(in.Contributors) > 50 {
		return nil, errors.New("contributors must contain 1-50 entries")
	}
	seen := make(map[string]bool)
	for _, c := range in.Contributors {
		if err := validateID(c.ArtisanID); err != nil {
			return nil, err
		}
		if err := nonEmpty(c.Role, "role"); err != nil {
			return nil, err
		}
		if seen[c.ArtisanID] {
			return nil, errors.New("an artisan may only appear once per activity")
		}
		seen[c.ArtisanID] = true
		if _, err := load[Artisan](ctx, "artisan", c.ArtisanID); err != nil {
			return nil, err
		}
	}
	if len(in.Materials) > 50 {
		return nil, errors.New("too many materials in one activity")
	}
	if err := uniqueInputs(in.Materials); err != nil {
		return nil, err
	}
	lots := make([]*MaterialLot, 0, len(in.Materials))
	for _, used := range in.Materials {
		lot, err := load[MaterialLot](ctx, "lot", used.LotID)
		if err != nil {
			return nil, err
		}
		if lot.CustodianMSP != caller.MSPID {
			return nil, fmt.Errorf("material %q is not in product owner's custody", used.LotID)
		}
		if used.QuantityGrams > lot.QuantityGrams-lot.UsedGrams {
			return nil, fmt.Errorf("insufficient material in lot %q", used.LotID)
		}
		lots = append(lots, lot)
	}
	a := &Activity{ID: in.ID, ProductID: in.ProductID, Stage: in.Stage, Technique: in.Technique, LocationCode: in.LocationCode, Contributors: in.Contributors, Materials: in.Materials, EvidenceSHA256: in.EvidenceSHA256, RecordedByMSP: caller.MSPID, RecordedTxID: ctx.GetTxID()}
	for i, lot := range lots {
		lot.UsedGrams += in.Materials[i].QuantityGrams
		if err := put(ctx, "lot", lot.ID, lot); err != nil {
			return nil, err
		}
	}
	product.ActivityIDs = append(product.ActivityIDs, in.ID)
	if err := put(ctx, "activity", in.ID, a); err != nil {
		return nil, err
	}
	if err := put(ctx, "product", product.ID, product); err != nil {
		return nil, err
	}
	return a, event(ctx, "ActivityRecorded", in.ID)
}

func (c *Contract) PublishProduct(ctx TxContext, productID string) (*Product, error) {
	caller, err := require(ctx, "producer")
	if err != nil {
		return nil, err
	}
	product, err := load[Product](ctx, "product", productID)
	if err != nil {
		return nil, err
	}
	if product.OwnerMSP != caller.MSPID {
		return nil, errors.New("cannot publish another organization's product")
	}
	if product.Status != "draft" {
		return nil, errors.New("product is already published")
	}
	if len(product.ActivityIDs) == 0 {
		return nil, errors.New("product needs at least one activity")
	}
	materialsRecorded := false
	for _, actID := range product.ActivityIDs {
		activity, err := load[Activity](ctx, "activity", actID)
		if err != nil {
			return nil, err
		}
		if len(activity.Materials) > 0 {
			materialsRecorded = true
		}
	}
	if !materialsRecorded {
		return nil, errors.New("product needs at least one material lot for traceability")
	}
	product.Status = "published"
	product.PublishedTxID = ctx.GetTxID()
	if err := put(ctx, "product", product.ID, product); err != nil {
		return nil, err
	}
	return product, event(ctx, "ProductPublished", product.ID)
}

// AttestActivity records an *auditor's assertion*; it is not automatic proof of
// the truth of the statement. An auditor from the producing MSP is rejected.
func (c *Contract) AttestActivity(ctx TxContext, payload string) (*Attestation, error) {
	caller, err := require(ctx, "auditor")
	if err != nil {
		return nil, err
	}
	var in AttestationInput
	if err := decode(payload, &in); err != nil {
		return nil, err
	}
	activity, err := load[Activity](ctx, "activity", in.ActivityID)
	if err != nil {
		return nil, err
	}
	if activity.RecordedByMSP == caller.MSPID {
		return nil, errors.New("auditor cannot attest activity of own MSP")
	}
	if in.Verdict != "supported" && in.Verdict != "disputed" {
		return nil, errors.New("verdict must be supported or disputed")
	}
	if !digestPattern.MatchString(in.EvidenceSHA256) {
		return nil, errors.New("attestation requires a SHA-256 evidence hash")
	}
	attestKey := key("attest", in.ActivityID+"~"+caller.MSPID)
	if exists, err := ctx.GetState(attestKey); err != nil {
		return nil, err
	} else if exists != nil {
		return nil, errors.New("this MSP has already attested this activity")
	}
	a := &Attestation{ActivityID: in.ActivityID, AuditorMSP: caller.MSPID, Verdict: in.Verdict, EvidenceSHA256: in.EvidenceSHA256, RecordedTxID: ctx.GetTxID()}
	serialized, err := json.Marshal(a)
	if err != nil {
		return nil, err
	}
	if err := ctx.PutState(attestKey, serialized); err != nil {
		return nil, err
	}
	activity.AuditorMSPs = append(activity.AuditorMSPs, caller.MSPID)
	sort.Strings(activity.AuditorMSPs)
	if err := put(ctx, "activity", activity.ID, activity); err != nil {
		return nil, err
	}
	return a, event(ctx, "ActivityAttested", in.ActivityID)
}

func (c *Contract) GetArtisan(ctx TxContext, id string) (*Artisan, error) {
	return load[Artisan](ctx, "artisan", id)
}
func (c *Contract) GetMaterial(ctx TxContext, id string) (*MaterialLot, error) {
	return load[MaterialLot](ctx, "lot", id)
}
func (c *Contract) GetProduct(ctx TxContext, id string) (*Product, error) {
	return load[Product](ctx, "product", id)
}
func (c *Contract) GetActivity(ctx TxContext, id string) (*Activity, error) {
	return load[Activity](ctx, "activity", id)
}

// GetPassport renders a safe, stable, item-level projection. Source-lot
// ancestry is followed recursively with a bounded depth and sorted output.
func (c *Contract) GetPassport(ctx TxContext, productID string) (*Passport, error) {
	p, err := load[Product](ctx, "product", productID)
	if err != nil {
		return nil, err
	}
	if p.Status != "published" {
		return nil, errors.New("passport is unavailable until product is published")
	}
	out := &Passport{ID: p.ID, Name: p.Name, Category: p.Category, Status: p.Status, Activities: []PublicActivity{}, Materials: []PublicMaterial{}}
	lotIDs := make(map[string]bool)
	for _, actID := range p.ActivityIDs {
		a, err := load[Activity](ctx, "activity", actID)
		if err != nil {
			return nil, err
		}
		pa := PublicActivity{ID: a.ID, Stage: a.Stage, Technique: a.Technique, LocationCode: a.LocationCode, EvidenceSHA256: a.EvidenceSHA256, Materials: a.Materials, Artisans: []PublicArtisan{}, Attestations: []Attestation{}}
		for _, contrib := range a.Contributors {
			artisan, err := load[Artisan](ctx, "artisan", contrib.ArtisanID)
			if err != nil {
				return nil, err
			}
			public := PublicArtisan{ID: artisan.ID, Role: contrib.Role}
			if artisan.Public {
				public.Alias = artisan.PublicAlias
			}
			pa.Artisans = append(pa.Artisans, public)
		}
		for _, used := range a.Materials {
			if err := walkLots(ctx, used.LotID, lotIDs, &out.Materials, 0); err != nil {
				return nil, err
			}
		}
		// Explicitly indexed attestations avoid nondeterministic range queries.
		for _, auditorMSP := range a.AuditorMSPs {
			attestation, err := c.GetAttestation(ctx, a.ID, auditorMSP)
			if err != nil {
				return nil, err
			}
			pa.Attestations = append(pa.Attestations, *attestation)
		}
		out.Activities = append(out.Activities, pa)
	}
	sort.Slice(out.Materials, func(i, j int) bool { return out.Materials[i].ID < out.Materials[j].ID })
	return out, nil
}

func walkLots(ctx TxContext, id string, seen map[string]bool, dest *[]PublicMaterial, depth int) error {
	if depth > 32 {
		return errors.New("material provenance depth exceeds 32")
	}
	if seen[id] {
		return nil
	}
	lot, err := load[MaterialLot](ctx, "lot", id)
	if err != nil {
		return err
	}
	seen[id] = true
	*dest = append(*dest, PublicMaterial{ID: lot.ID, MaterialType: lot.MaterialType, OriginRegion: lot.OriginRegion, SourceLots: lot.SourceLots, EvidenceSHA256: lot.EvidenceSHA256})
	for _, src := range lot.SourceLots {
		if err := walkLots(ctx, src.LotID, seen, dest, depth+1); err != nil {
			return err
		}
	}
	return nil
}

// GetAttestation is an independent query: full ledger access is not controlled
// by the public JSON projection, and on-chain evidence hashes remain visible.
func (c *Contract) GetAttestation(ctx TxContext, activityID, auditorMSP string) (*Attestation, error) {
	if err := validateID(activityID); err != nil {
		return nil, err
	}
	if err := nonEmpty(auditorMSP, "auditorMSP"); err != nil {
		return nil, err
	}
	encoded, err := ctx.GetState(key("attest", activityID+"~"+auditorMSP))
	if err != nil {
		return nil, err
	}
	if encoded == nil {
		return nil, errors.New("attestation not found")
	}
	var a Attestation
	if err := json.Unmarshal(encoded, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// VerifyEvidence checks a caller-provided SHA-256 digest against the hash on
// a recorded activity. It does NOT authenticate who performed the activity.
func (c *Contract) VerifyEvidence(ctx TxContext, activityID, digest string) (bool, error) {
	if _, err := hex.DecodeString(digest); err != nil || !digestPattern.MatchString(digest) {
		return false, errors.New("invalid SHA-256 hex digest")
	}
	a, err := load[Activity](ctx, "activity", activityID)
	if err != nil {
		return false, err
	}
	return a.EvidenceSHA256 != "" && bytes.Equal([]byte(a.EvidenceSHA256), []byte(digest)), nil
}
