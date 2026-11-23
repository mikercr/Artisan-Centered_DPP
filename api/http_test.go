package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"example.com/artisan-dpp-fabric/backend"
)

func setup(t *testing.T, token string) http.Handler {
	t.Helper()
	b, err := backend.NewDemo("", true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = b.Close() })
	return New(b, token, nil)
}
func req(h http.Handler, method, path, body, actor, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewBufferString(body))
	if body != "" {
		r.Header.Set("Content-Type", "application/json")
	}
	if actor != "" {
		r.Header.Set("X-Actor", actor)
	}
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("got %d want %d: %s", w.Code, want, w.Body.String())
	}
}
func TestPublicPassportAndMetadata(t *testing.T) {
	h := setup(t, "very-long-token-value")
	assertStatus(t, req(h, "GET", "/api/health", "", "", ""), 200)
	assertStatus(t, req(h, "GET", "/api/meta", "", "", ""), 200)
	p := req(h, "GET", "/api/passports/vase-001", "", "public-reader", "")
	assertStatus(t, p, 200)
	var data struct {
		Data struct {
			ID         string            `json:"id"`
			Activities []json.RawMessage `json:"activities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(p.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if data.Data.ID != "vase-001" || len(data.Data.Activities) != 3 {
		t.Fatalf("unexpected passport: %s", p.Body.String())
	}
	if strings.Contains(p.Body.String(), "artisan-hidden\",\"alias") {
		t.Fatalf("leaked hidden alias")
	}
}
func TestAuthenticatedWriteAndRoleEnforcement(t *testing.T) {
	h := setup(t, "correct-token")
	body := `{"id":"vase-api","name":"Handmade bowl","category":"Ceramics"}`
	assertStatus(t, req(h, "POST", "/api/products", body, "workshop-producer", ""), 401)
	assertStatus(t, req(h, "POST", "/api/products", body, "public-reader", "correct-token"), 403)
	w := req(h, "POST", "/api/products", body, "workshop-producer", "correct-token")
	assertStatus(t, w, 201)
	assertStatus(t, req(h, "GET", "/api/products/vase-api", "", "workshop-producer", ""), 401)
	assertStatus(t, req(h, "GET", "/api/products/vase-api", "", "workshop-producer", "correct-token"), 200)
	assertStatus(t, req(h, "POST", "/api/products", body, "workshop-producer", "correct-token"), 409)
}
func TestTransferAndMaterialDerivation(t *testing.T) {
	h := setup(t, "")
	transfer := req(h, "POST", "/api/materials/clay-alentejo-01/transfer", `{"toMSP":"SupplierMSP"}`, "workshop-producer", "")
	assertStatus(t, transfer, 201)
	rec := req(h, "GET", "/api/materials/clay-alentejo-01", "", "workshop-producer", "")
	assertStatus(t, rec, 200)
	if !strings.Contains(rec.Body.String(), "SupplierMSP") {
		t.Fatalf("missing custody transfer: %s", rec.Body.String())
	}
	derive := req(h, "POST", "/api/materials/derive", `{"id":"derived-2026","materialType":"Refined clay","quantityGrams":500,"sourceLots":[{"lotId":"clay-alentejo-01","quantityGrams":600}]}`, "supplier-registrar", "")
	assertStatus(t, derive, 201)
	assertStatus(t, req(h, "GET", "/api/materials/derived-2026", "", "", ""), 200)
}
func TestBodyValidationAndLimits(t *testing.T) {
	h := setup(t, "")
	assertStatus(t, req(h, "POST", "/api/artisans", "[1]", "workshop-registrar", ""), 400)
	assertStatus(t, req(h, "POST", "/api/artisans", "not-json", "workshop-registrar", ""), 400)
	assertStatus(t, req(h, "POST", "/api/artisans", `{"id":"new","public":true}`+strings.Repeat(" ", 65536), "workshop-registrar", ""), 400)
}
func TestDemoCatalogAndFabricLimit(t *testing.T) {
	h := setup(t, "")
	w := req(h, "GET", "/api/records/products", "", "workshop-producer", "")
	assertStatus(t, w, 200)
	if !strings.Contains(w.Body.String(), "vase-001") {
		t.Fatalf("sample product missing")
	}
	assertStatus(t, req(h, "GET", "/api/records/unknown", "", "workshop-producer", ""), 400)
}
