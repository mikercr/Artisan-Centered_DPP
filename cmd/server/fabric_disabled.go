//go:build !fabric

package main

import (
	"errors"
	"example.com/artisan-dpp-fabric/backend"
)

func newFabricBackend() (backend.Backend, error) {
	return nil, errors.New("Fabric Gateway support is not in this build; compile with -tags fabric after downloading SDK dependencies")
}
