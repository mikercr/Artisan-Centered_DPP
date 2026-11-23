package main

import (
	"testing"

	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

// Run on a machine that can fetch the Fabric Contract API modules:
//
//	go mod tidy && go test ./cmd/chaincode -v
//
// It checks that reflection-based transaction registration accepts all
// exported methods and their argument/return types.
func TestFabricContractAPIRegistration(t *testing.T) {
	if _, err := contractapi.NewChaincode(&SmartContract{}); err != nil {
		t.Fatalf("failed to register chaincode: %v", err)
	}
}
