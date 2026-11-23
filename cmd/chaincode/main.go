// Command chaincode exposes the domain contract through Hyperledger Fabric's
// official Go Contract API (v1.2.2, compatible with Go 1.23).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"

	"example.com/artisan-dpp-fabric/dpp"
	"github.com/hyperledger/fabric-contract-api-go/contractapi"
)

type SmartContract struct{ contractapi.Contract }

type fabricTx struct {
	ctx contractapi.TransactionContextInterface
}

func (f *fabricTx) GetState(key string) ([]byte, error)   { return f.ctx.GetStub().GetState(key) }
func (f *fabricTx) PutState(key string, val []byte) error { return f.ctx.GetStub().PutState(key, val) }
func (f *fabricTx) GetTxID() string                       { return f.ctx.GetStub().GetTxID() }
func (f *fabricTx) EmitEvent(name string, payload []byte) error {
	return f.ctx.GetStub().SetEvent(name, payload)
}
func (f *fabricTx) Caller() (dpp.Caller, error) {
	id := f.ctx.GetClientIdentity()
	msp, err := id.GetMSPID()
	if err != nil {
		return dpp.Caller{}, fmt.Errorf("reading Fabric MSP: %w", err)
	}
	role, found, err := id.GetAttributeValue("dpp.role")
	if err != nil {
		return dpp.Caller{}, fmt.Errorf("reading dpp.role certificate attribute: %w", err)
	}
	if !found {
		role = "reader"
	}
	clientID, err := id.GetID()
	if err != nil {
		return dpp.Caller{}, fmt.Errorf("reading Fabric client identity: %w", err)
	}
	digest := sha256.Sum256([]byte(clientID))
	return dpp.Caller{MSPID: msp, Role: role, IDHash: hex.EncodeToString(digest[:])}, nil
}

func (s *SmartContract) RegisterArtisan(ctx contractapi.TransactionContextInterface, payload string) (*dpp.Artisan, error) {
	return (&dpp.Contract{}).RegisterArtisan(&fabricTx{ctx}, payload)
}
func (s *SmartContract) RegisterMaterial(ctx contractapi.TransactionContextInterface, payload string) (*dpp.MaterialLot, error) {
	return (&dpp.Contract{}).RegisterMaterial(&fabricTx{ctx}, payload)
}
func (s *SmartContract) TransferMaterial(ctx contractapi.TransactionContextInterface, lotID, toMSP string) (*dpp.MaterialLot, error) {
	return (&dpp.Contract{}).TransferMaterial(&fabricTx{ctx}, lotID, toMSP)
}
func (s *SmartContract) DeriveMaterial(ctx contractapi.TransactionContextInterface, payload string) (*dpp.MaterialLot, error) {
	return (&dpp.Contract{}).DeriveMaterial(&fabricTx{ctx}, payload)
}
func (s *SmartContract) CreateProduct(ctx contractapi.TransactionContextInterface, payload string) (*dpp.Product, error) {
	return (&dpp.Contract{}).CreateProduct(&fabricTx{ctx}, payload)
}
func (s *SmartContract) RecordActivity(ctx contractapi.TransactionContextInterface, payload string) (*dpp.Activity, error) {
	return (&dpp.Contract{}).RecordActivity(&fabricTx{ctx}, payload)
}
func (s *SmartContract) PublishProduct(ctx contractapi.TransactionContextInterface, productID string) (*dpp.Product, error) {
	return (&dpp.Contract{}).PublishProduct(&fabricTx{ctx}, productID)
}
func (s *SmartContract) AttestActivity(ctx contractapi.TransactionContextInterface, payload string) (*dpp.Attestation, error) {
	return (&dpp.Contract{}).AttestActivity(&fabricTx{ctx}, payload)
}
func (s *SmartContract) GetArtisan(ctx contractapi.TransactionContextInterface, id string) (*dpp.Artisan, error) {
	return (&dpp.Contract{}).GetArtisan(&fabricTx{ctx}, id)
}
func (s *SmartContract) GetMaterial(ctx contractapi.TransactionContextInterface, id string) (*dpp.MaterialLot, error) {
	return (&dpp.Contract{}).GetMaterial(&fabricTx{ctx}, id)
}
func (s *SmartContract) GetProduct(ctx contractapi.TransactionContextInterface, id string) (*dpp.Product, error) {
	return (&dpp.Contract{}).GetProduct(&fabricTx{ctx}, id)
}
func (s *SmartContract) GetActivity(ctx contractapi.TransactionContextInterface, id string) (*dpp.Activity, error) {
	return (&dpp.Contract{}).GetActivity(&fabricTx{ctx}, id)
}
func (s *SmartContract) GetAttestation(ctx contractapi.TransactionContextInterface, activityID, auditorMSP string) (*dpp.Attestation, error) {
	return (&dpp.Contract{}).GetAttestation(&fabricTx{ctx}, activityID, auditorMSP)
}
func (s *SmartContract) GetPassport(ctx contractapi.TransactionContextInterface, productID string) (*dpp.Passport, error) {
	return (&dpp.Contract{}).GetPassport(&fabricTx{ctx}, productID)
}
func (s *SmartContract) VerifyEvidence(ctx contractapi.TransactionContextInterface, activityID, digest string) (bool, error) {
	return (&dpp.Contract{}).VerifyEvidence(&fabricTx{ctx}, activityID, digest)
}

func main() {
	chaincode, err := contractapi.NewChaincode(&SmartContract{})
	if err != nil {
		log.Fatalf("unable to create chaincode: %v", err)
	}
	if err := chaincode.Start(); err != nil {
		log.Fatalf("unable to start chaincode: %v", err)
	}
}
