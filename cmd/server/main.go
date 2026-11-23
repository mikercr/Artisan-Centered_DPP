package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"example.com/artisan-dpp-fabric/api"
	"example.com/artisan-dpp-fabric/backend"
)

//go:embed web/* web/assets/*
var webFiles embed.FS

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
func main() {
	mode := env("DPP_BACKEND", "demo")
	var b backend.Backend
	var err error
	switch mode {
	case "demo":
		b, err = backend.NewDemo(os.Getenv("DEMO_STATE_FILE"), env("DEMO_SEED", "true") != "false")
	case "fabric":
		b, err = newFabricBackend()
	default:
		log.Fatalf("unknown DPP_BACKEND=%q", mode)
	}
	if err != nil {
		log.Fatal(err)
	}
	defer b.Close()
	token := os.Getenv("API_TOKEN")
	if mode == "fabric" && len(token) < 32 {
		log.Fatal("fabric mode requires API_TOKEN of at least 32 characters (also restrict network access and use HTTPS)")
	}
	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	host := env("DPP_LISTEN", "127.0.0.1:8080")
	if !strings.HasPrefix(host, "127.0.0.1:") && !strings.HasPrefix(host, "localhost:") && !strings.HasPrefix(host, "[::1]:") {
		log.Println("WARNING: listening on a non-loopback address; configure HTTPS reverse proxy, authentication and access controls")
	}
	server := &http.Server{Addr: host, Handler: api.New(b, token, static), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 35 * time.Second, IdleTimeout: 60 * time.Second}
	log.Printf("CraftPass Studio (%s) listening at http://%s", mode, host)
	log.Fatal(server.ListenAndServe())
}
