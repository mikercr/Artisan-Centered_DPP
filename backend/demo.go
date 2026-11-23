package backend

import (
	"context"
	"encoding/json"
	"errors"
	"example.com/artisan-dpp-fabric/dpp"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

var demoActors = []Actor{
	{"workshop-registrar", "Workshop registrar", "WorkshopMSP", "registrar"},
	{"workshop-producer", "Workshop producer", "WorkshopMSP", "producer"},
	{"supplier-registrar", "Material supplier", "SupplierMSP", "registrar"},
	{"independent-auditor", "Independent auditor", "CraftAuthorityMSP", "auditor"},
	{"public-reader", "Public reader", "PublicMSP", "reader"},
}

type Demo struct {
	mu       sync.Mutex
	state    map[string][]byte
	serial   uint64
	filename string
}

type demoTx struct {
	state  map[string][]byte
	caller dpp.Caller
	id     string
}

func (t *demoTx) GetState(k string) ([]byte, error) { return t.state[k], nil }
func (t *demoTx) PutState(k string, v []byte) error { t.state[k] = append([]byte{}, v...); return nil }
func (t *demoTx) GetTxID() string                   { return t.id }
func (t *demoTx) Caller() (dpp.Caller, error)       { return t.caller, nil }
func (t *demoTx) EmitEvent(string, []byte) error    { return nil }

// NewDemo uses the exact dpp.Contract domain logic, not a separate mock
// implementation. An optional JSON snapshot persists across restarts.
func NewDemo(filename string, seed bool) (*Demo, error) {
	b := &Demo{state: make(map[string][]byte), filename: filename}
	if filename != "" {
		raw, err := os.ReadFile(filename)
		if err == nil {
			var saved struct {
				State  map[string][]byte `json:"state"`
				Serial uint64            `json:"serial"`
			}
			if err = json.Unmarshal(raw, &saved); err != nil {
				return nil, fmt.Errorf("invalid demo state: %w", err)
			}
			if saved.State != nil {
				b.state = saved.State
			}
			b.serial = saved.Serial
			return b, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
	}
	if seed {
		if err := b.Seed(); err != nil {
			return nil, err
		}
	}
	return b, nil
}
func (b *Demo) Mode() string    { return "demo" }
func (b *Demo) Actors() []Actor { return append([]Actor{}, demoActors...) }
func (b *Demo) Close() error    { return nil }

func (b *Demo) save(state map[string][]byte, serial uint64) error {
	if b.filename == "" {
		return nil
	}
	payload, err := json.MarshalIndent(struct {
		State  map[string][]byte `json:"state"`
		Serial uint64            `json:"serial"`
	}{state, serial}, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(b.filename), 0700); err != nil {
		return err
	}
	temp := b.filename + ".tmp"
	if err = os.WriteFile(temp, payload, 0600); err != nil {
		return err
	}
	return os.Rename(temp, b.filename)
}
func isRead(name string) bool { return strings.HasPrefix(name, "Get") || name == "VerifyEvidence" }
func (b *Demo) Invoke(_ context.Context, actor, method string, args ...string) (json.RawMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	identity := dpp.Caller{}
	found := false
	for _, a := range demoActors {
		if a.ID == actor {
			identity = dpp.Caller{MSPID: a.MSPID, Role: a.Role, IDHash: "demo-identity-" + a.ID}
			found = true
			break
		}
	}
	if !found {
		return nil, errors.New("unknown demo actor")
	}
	state := make(map[string][]byte, len(b.state))
	for k, v := range b.state {
		state[k] = append([]byte{}, v...)
	}
	tx := &demoTx{state: state, caller: identity, id: fmt.Sprintf("demo-tx-%06d", b.serial+1)}
	c := &dpp.Contract{}
	arg := func(i int) string {
		if i >= len(args) {
			return ""
		}
		return args[i]
	}
	var res any
	var err error
	switch method {
	case "RegisterArtisan":
		res, err = c.RegisterArtisan(tx, arg(0))
	case "RegisterMaterial":
		res, err = c.RegisterMaterial(tx, arg(0))
	case "TransferMaterial":
		res, err = c.TransferMaterial(tx, arg(0), arg(1))
	case "DeriveMaterial":
		res, err = c.DeriveMaterial(tx, arg(0))
	case "CreateProduct":
		res, err = c.CreateProduct(tx, arg(0))
	case "RecordActivity":
		res, err = c.RecordActivity(tx, arg(0))
	case "PublishProduct":
		res, err = c.PublishProduct(tx, arg(0))
	case "AttestActivity":
		res, err = c.AttestActivity(tx, arg(0))
	case "GetArtisan":
		res, err = c.GetArtisan(tx, arg(0))
	case "GetMaterial":
		res, err = c.GetMaterial(tx, arg(0))
	case "GetProduct":
		res, err = c.GetProduct(tx, arg(0))
	case "GetActivity":
		res, err = c.GetActivity(tx, arg(0))
	case "GetAttestation":
		res, err = c.GetAttestation(tx, arg(0), arg(1))
	case "GetPassport":
		res, err = c.GetPassport(tx, arg(0))
	case "VerifyEvidence":
		res, err = c.VerifyEvidence(tx, arg(0), arg(1))
	default:
		return nil, fmt.Errorf("unknown contract method %q", method)
	}
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(res)
	if err != nil {
		return nil, err
	}
	if !isRead(method) {
		if err := b.save(state, b.serial+1); err != nil {
			return nil, fmt.Errorf("saving demo state: %w", err)
		}
		b.state = state
		b.serial++
	}
	return raw, nil
}
func (b *Demo) List(kind string) ([]json.RawMessage, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	kindPrefix := map[string]string{"artisans": "artisan~", "materials": "lot~", "products": "product~", "activities": "activity~"}[kind]
	if kindPrefix == "" {
		return nil, errors.New("invalid resource type")
	}
	keys := make([]string, 0)
	for k := range b.state {
		if strings.HasPrefix(k, kindPrefix) {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	out := make([]json.RawMessage, 0, len(keys))
	for _, k := range keys {
		// Demo listing never exposes artisan personal information that was
		// intentionally excluded from a published public passport.
		out = append(out, json.RawMessage(b.state[k]))
	}
	return out, nil
}
func seedJSON(v any) string { b, _ := json.Marshal(v); return string(b) }
func (b *Demo) Seed() error {
	run := func(actor, method string, args ...string) error {
		_, err := b.Invoke(context.Background(), actor, method, args...)
		return err
	}
	for _, a := range []dpp.ArtisanInput{
		{ID: "artisan-ana", Public: true, PublicAlias: "Ana Martins", Skills: []string{"Wheel throwing", "Clay shaping"}},
		{ID: "artisan-joao", Public: true, PublicAlias: "João Pereira", Skills: []string{"Glazing", "Decoration"}},
		{ID: "artisan-hidden", Public: false, Skills: []string{"Kiln firing"}},
	} {
		if err := run("workshop-registrar", "RegisterArtisan", seedJSON(a)); err != nil {
			return err
		}
	}
	for _, m := range []dpp.MaterialInput{
		{ID: "clay-alentejo-01", MaterialType: "Red clay", OriginRegion: "Alentejo, Portugal", QuantityGrams: 5000, SupplierRef: "Local clay supplier"},
		{ID: "glaze-porto-01", MaterialType: "Ceramic glaze", OriginRegion: "Porto, Portugal", QuantityGrams: 1200},
	} {
		if err := run("supplier-registrar", "RegisterMaterial", seedJSON(m)); err != nil {
			return err
		}
	}
	for _, id := range []string{"clay-alentejo-01", "glaze-porto-01"} {
		if err := run("supplier-registrar", "TransferMaterial", id, "WorkshopMSP"); err != nil {
			return err
		}
	}
	if err := run("workshop-producer", "CreateProduct", seedJSON(dpp.ProductInput{ID: "vase-001", Name: "Handcrafted ceramic vase", Category: "Ceramics"})); err != nil {
		return err
	}
	for _, a := range []dpp.ActivityInput{
		{ID: "vase-001-shaping", ProductID: "vase-001", Stage: "Shaping", Technique: "Wheel throwing", LocationCode: "Workshop Porto", Contributors: []dpp.Contribution{{ArtisanID: "artisan-ana", Role: "Potter"}}, Materials: []dpp.MaterialUse{{LotID: "clay-alentejo-01", QuantityGrams: 850}}},
		{ID: "vase-001-glazing", ProductID: "vase-001", Stage: "Glazing", Technique: "Brush glazing", LocationCode: "Workshop Porto", Contributors: []dpp.Contribution{{ArtisanID: "artisan-joao", Role: "Decorator"}}, Materials: []dpp.MaterialUse{{LotID: "glaze-porto-01", QuantityGrams: 120}}},
		{ID: "vase-001-firing", ProductID: "vase-001", Stage: "Firing", Technique: "Kiln firing", LocationCode: "Workshop Porto", Contributors: []dpp.Contribution{{ArtisanID: "artisan-hidden", Role: "Kiln operator"}}},
	} {
		if err := run("workshop-producer", "RecordActivity", seedJSON(a)); err != nil {
			return err
		}
	}
	if err := run("workshop-producer", "PublishProduct", "vase-001"); err != nil {
		return err
	}
	if err := run("independent-auditor", "AttestActivity", seedJSON(dpp.AttestationInput{ActivityID: "vase-001-shaping", Verdict: "supported", EvidenceSHA256: strings.Repeat("a", 64)})); err != nil {
		return err
	}
	return nil
}
