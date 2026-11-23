//go:build fabric

package main

import (
	"errors"
	"example.com/artisan-dpp-fabric/backend"
	"os"
)

func newFabricBackend() (backend.Backend, error) {
	p := os.Getenv("FABRIC_CONFIG")
	if p == "" {
		return nil, errors.New("FABRIC_CONFIG must point to server-side Fabric identities file")
	}
	return backend.NewFabric(p)
}
