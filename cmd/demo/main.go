package main

import (
	"encoding/json"
	"fmt"
	"os"

	"example.com/artisan-dpp-fabric/dpp"
)

// An offline illustration that exercises the same domain operations as the
// Fabric adapter. It does not simulate endorsement or validation/commit.
type memory struct {
	state     map[string][]byte
	tx        int
	msp, role string
}

func (m *memory) GetState(key string) ([]byte, error)   { return m.state[key], nil }
func (m *memory) PutState(key string, val []byte) error { m.state[key] = val; return nil }
func (m *memory) GetTxID() string                       { return fmt.Sprintf("example-%02d", m.tx) }
func (m *memory) EmitEvent(_ string, _ []byte) error    { return nil }
func (m *memory) Caller() (dpp.Caller, error) {
	return dpp.Caller{MSPID: m.msp, Role: m.role, IDHash: "example-fabric-id-hash"}, nil
}
func (m *memory) set(msp, role string) { m.tx++; m.msp, m.role = msp, role }
func payload(x interface{}) string     { b, _ := json.Marshal(x); return string(b) }
func must(err error) {
	if err != nil {
		panic(err)
	}
}
func main() {
	m := &memory{state: make(map[string][]byte)}
	c := &dpp.Contract{}
	m.set("WorkshopMSP", "registrar")
	_, err := c.RegisterArtisan(m, payload(dpp.ArtisanInput{ID: "artisan-04", Public: true, PublicAlias: "Ana", Skills: []string{"ceramics"}}))
	must(err)
	m.set("WorkshopMSP", "registrar")
	_, err = c.RegisterArtisan(m, payload(dpp.ArtisanInput{ID: "artisan-05", Public: false, Skills: []string{"kiln operation"}}))
	must(err)
	m.set("SupplierMSP", "registrar")
	_, err = c.RegisterMaterial(m, payload(dpp.MaterialInput{ID: "clay-batch-08", MaterialType: "red clay", OriginRegion: "Alentejo, Portugal", QuantityGrams: 3000}))
	must(err)
	m.set("SupplierMSP", "registrar")
	_, err = c.TransferMaterial(m, "clay-batch-08", "WorkshopMSP")
	must(err)
	m.set("WorkshopMSP", "producer")
	_, err = c.CreateProduct(m, payload(dpp.ProductInput{ID: "vase-023", Name: "Hand-thrown ceramic vase", Category: "ceramics"}))
	must(err)
	m.set("WorkshopMSP", "producer")
	_, err = c.RecordActivity(m, payload(dpp.ActivityInput{ID: "shaping-17", ProductID: "vase-023", Stage: "shaping", Technique: "wheel throwing", Contributors: []dpp.Contribution{{ArtisanID: "artisan-04", Role: "potter"}}, Materials: []dpp.MaterialUse{{LotID: "clay-batch-08", QuantityGrams: 800}}}))
	must(err)
	m.set("WorkshopMSP", "producer")
	_, err = c.RecordActivity(m, payload(dpp.ActivityInput{ID: "firing-09", ProductID: "vase-023", Stage: "firing", Technique: "kiln firing", Contributors: []dpp.Contribution{{ArtisanID: "artisan-05", Role: "kiln operator"}}}))
	must(err)
	m.set("WorkshopMSP", "producer")
	_, err = c.PublishProduct(m, "vase-023")
	must(err)
	m.set("PublicMSP", "reader")
	passport, err := c.GetPassport(m, "vase-023")
	must(err)
	out, err := json.MarshalIndent(passport, "", "  ")
	must(err)
	fmt.Fprintln(os.Stdout, string(out))
}
