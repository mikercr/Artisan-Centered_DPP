package api

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"example.com/artisan-dpp-fabric/backend"
)

type Server struct {
	backend backend.Backend
	token   string
	static  fs.FS
}

func New(b backend.Backend, token string, static fs.FS) http.Handler {
	s := &Server{backend: b, token: token, static: static}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.health)
	mux.HandleFunc("GET /api/meta", s.meta)
	mux.HandleFunc("GET /api/passports/{id}", s.passport)
	mux.HandleFunc("GET /api/records/{kind}", s.records)
	mux.HandleFunc("GET /api/artisans/{id}", s.query("GetArtisan"))
	mux.HandleFunc("GET /api/materials/{id}", s.query("GetMaterial"))
	mux.HandleFunc("GET /api/products/{id}", s.query("GetProduct"))
	mux.HandleFunc("GET /api/activities/{id}", s.query("GetActivity"))
	mux.HandleFunc("GET /api/activities/{id}/attestations/{msp}", s.query("GetAttestation"))
	mux.HandleFunc("GET /api/activities/{id}/evidence/{sha256}", s.query("VerifyEvidence"))
	mux.HandleFunc("POST /api/artisans", s.create("RegisterArtisan"))
	mux.HandleFunc("POST /api/materials", s.create("RegisterMaterial"))
	mux.HandleFunc("POST /api/materials/derive", s.create("DeriveMaterial"))
	mux.HandleFunc("POST /api/materials/{id}/transfer", s.transfer)
	mux.HandleFunc("POST /api/products", s.create("CreateProduct"))
	mux.HandleFunc("POST /api/activities", s.create("RecordActivity"))
	mux.HandleFunc("POST /api/products/{id}/publish", s.publish)
	mux.HandleFunc("POST /api/attestations", s.create("AttestActivity"))
	if static != nil {
		files := http.FileServer(http.FS(static))
		mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/" {
				http.NotFound(w, r)
				return
			}
			page, err := fs.ReadFile(static, "index.html")
			if err != nil {
				http.Error(w, "web UI is unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(page)
		})
		mux.Handle("GET /assets/", files)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		mux.ServeHTTP(w, r)
	})
}
func send(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, msg string) {
	send(w, status, map[string]string{"error": msg})
}
func (s *Server) authenticated(r *http.Request) bool {
	if s.token == "" && s.backend.Mode() == "demo" {
		return true
	}
	bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	return s.token != "" && len(bearer) == len(s.token) && subtle.ConstantTimeCompare([]byte(bearer), []byte(s.token)) == 1
}
func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if s.authenticated(r) {
		return true
	}
	fail(w, http.StatusUnauthorized, "API bearer token is required")
	return false
}
func (s *Server) actor(r *http.Request) string {
	a := r.Header.Get("X-Actor")
	if a == "" && s.backend.Mode() == "demo" {
		return "workshop-producer"
	}
	return a
}
func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	send(w, 200, map[string]any{"status": "ok", "backend": s.backend.Mode()})
}
func (s *Server) meta(w http.ResponseWriter, _ *http.Request) {
	send(w, 200, map[string]any{"mode": s.backend.Mode(), "actors": s.backend.Actors(), "requiresToken": s.token != "" || s.backend.Mode() != "demo", "publicPassport": true, "listingSupported": s.backend.Mode() == "demo"})
}
func statusFor(err error) int {
	if err == nil {
		return 200
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "access denied"), strings.Contains(s, "cannot modify another"), strings.Contains(s, "cannot publish another"), strings.Contains(s, "only current material custodian"), strings.Contains(s, "not in product owner's custody"), strings.Contains(s, "cannot consume non-custodied"), strings.Contains(s, "auditor cannot"):
		return 403
	case strings.Contains(s, "not found"):
		return 404
	case strings.Contains(s, "already exists"), strings.Contains(s, "already published"), strings.Contains(s, "already attested"), strings.Contains(s, "insufficient"), strings.Contains(s, "until product is published"):
		return 409
	case strings.Contains(s, "fabric"), strings.Contains(s, "gateway"), strings.Contains(s, "unavailable"):
		return 502
	default:
		return 400
	}
}
func (s *Server) call(w http.ResponseWriter, r *http.Request, method string, args ...string) {
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := s.backend.Invoke(ctx, s.actor(r), method, args...)
	if err != nil {
		fail(w, statusFor(err), err.Error())
		return
	}
	status := 200
	if r.Method == http.MethodPost {
		status = 201
	}
	send(w, status, map[string]any{"data": json.RawMessage(result)})
}
func (s *Server) create(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(w, r) {
			return
		}
		// Payload is deliberately forwarded unchanged (after a single strict
		// JSON-value check). The contract applies domain-specific validation.
		raw, err := readJSON(r)
		if err != nil {
			fail(w, 400, err.Error())
			return
		}
		s.call(w, r, method, string(raw))
	}
}
func readJSON(r *http.Request) ([]byte, error) {
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return nil, errors.New("Content-Type must be application/json")
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 65537))
	if err != nil {
		return nil, err
	}
	if len(data) > 65536 {
		return nil, errors.New("body exceeds 64 KiB")
	}
	if len(data) == 0 || !json.Valid(data) {
		return nil, errors.New("invalid JSON payload")
	}
	// Do not allow a JSON array/primitive as a contract payload.
	if len(strings.TrimSpace(string(data))) == 0 || strings.TrimSpace(string(data))[0] != '{' {
		return nil, errors.New("JSON object required")
	}
	return data, nil
}
func (s *Server) query(method string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authorize(w, r) {
			return
		}
		args := []string{r.PathValue("id")}
		if method == "GetAttestation" {
			args = append(args, r.PathValue("msp"))
		}
		if method == "VerifyEvidence" {
			args = append(args, r.PathValue("sha256"))
		}
		s.call(w, r, method, args...)
	}
}
func (s *Server) passport(w http.ResponseWriter, r *http.Request) {
	// The domain projection excludes non-public aliases. This is the only
	// unrestricted ledger-backed read route for deployed applications.
	s.call(w, r, "GetPassport", r.PathValue("id"))
}
func (s *Server) transfer(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
	raw, err := readJSON(r)
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	var in struct {
		ToMSP string `json:"toMSP"`
	}
	if err = json.Unmarshal(raw, &in); err != nil {
		fail(w, 400, "invalid transfer JSON")
		return
	}
	s.call(w, r, "TransferMaterial", r.PathValue("id"), in.ToMSP)
}
func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
	s.call(w, r, "PublishProduct", r.PathValue("id"))
}
func (s *Server) records(w http.ResponseWriter, r *http.Request) {
	if !s.authorize(w, r) {
		return
	}
	catalog, ok := s.backend.(backend.Catalog)
	if !ok {
		fail(w, 501, "ledger enumeration is not implemented in the original Fabric chaincode; query records by ID")
		return
	}
	data, err := catalog.List(r.PathValue("kind"))
	if err != nil {
		fail(w, 400, err.Error())
		return
	}
	send(w, 200, map[string]any{"data": data})
}
