//go:build fabric

package backend

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hyperledger/fabric-gateway/pkg/client"
	"github.com/hyperledger/fabric-gateway/pkg/hash"
	"github.com/hyperledger/fabric-gateway/pkg/identity"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type fabricProfile struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	MSPID    string `json:"mspId"`
	Role     string `json:"role"` // display only; real role comes from enrollment certificate
	CertPath string `json:"certPath"`
	KeyPath  string `json:"keyPath"`
}

type fabricConfig struct {
	PeerEndpoint  string          `json:"peerEndpoint"`
	TLSServerName string          `json:"tlsServerName"`
	TLSCAPath     string          `json:"tlsCAPath"`
	Channel       string          `json:"channel"`
	Chaincode     string          `json:"chaincode"`
	Profiles      []fabricProfile `json:"profiles"`
}

type Fabric struct {
	conn      *grpc.ClientConn
	gateways  []*client.Gateway
	contracts map[string]*client.Contract
	actors    []Actor
}

// NewFabric creates a Gateway connection per configured X.509 identity.
// The browser chooses only among explicitly provisioned server-side certs;
// the client cannot impersonate an arbitrary MSP or Fabric role.
func NewFabric(configPath string) (*Fabric, error) {
	raw, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	var conf fabricConfig
	if err = json.Unmarshal(raw, &conf); err != nil {
		return nil, err
	}
	if conf.PeerEndpoint == "" || conf.TLSServerName == "" || conf.TLSCAPath == "" || conf.Channel == "" || conf.Chaincode == "" || len(conf.Profiles) == 0 {
		return nil, errors.New("missing required fields in Fabric configuration")
	}
	caPEM, err := os.ReadFile(conf.TLSCAPath)
	if err != nil {
		return nil, err
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("invalid TLS CA PEM")
	}
	conn, err := grpc.NewClient(conf.PeerEndpoint, grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(roots, conf.TLSServerName)))
	if err != nil {
		return nil, err
	}
	f := &Fabric{conn: conn, contracts: make(map[string]*client.Contract)}
	for _, p := range conf.Profiles {
		if p.ID == "" || p.MSPID == "" || p.CertPath == "" || p.KeyPath == "" {
			f.Close()
			return nil, errors.New("Fabric profile is missing required fields")
		}
		if _, exists := f.contracts[p.ID]; exists {
			f.Close()
			return nil, fmt.Errorf("duplicate Fabric profile %q", p.ID)
		}
		certPEM, err := os.ReadFile(p.CertPath)
		if err != nil {
			f.Close()
			return nil, err
		}
		cert, err := identity.CertificateFromPEM(certPEM)
		if err != nil {
			f.Close()
			return nil, err
		}
		id, err := identity.NewX509Identity(p.MSPID, cert)
		if err != nil {
			f.Close()
			return nil, err
		}
		privatePEM, err := os.ReadFile(p.KeyPath)
		if err != nil {
			f.Close()
			return nil, err
		}
		key, err := identity.PrivateKeyFromPEM(privatePEM)
		if err != nil {
			f.Close()
			return nil, err
		}
		signer, err := identity.NewPrivateKeySign(key)
		if err != nil {
			f.Close()
			return nil, err
		}
		gw, err := client.Connect(id, client.WithSign(signer), client.WithHash(hash.SHA256), client.WithClientConnection(conn), client.WithEvaluateTimeout(7*time.Second), client.WithEndorseTimeout(15*time.Second), client.WithSubmitTimeout(7*time.Second), client.WithCommitStatusTimeout(30*time.Second))
		if err != nil {
			f.Close()
			return nil, err
		}
		f.gateways = append(f.gateways, gw)
		f.contracts[p.ID] = gw.GetNetwork(conf.Channel).GetContract(conf.Chaincode)
		f.actors = append(f.actors, Actor{p.ID, p.Label, p.MSPID, p.Role})
	}
	return f, nil
}
func (f *Fabric) Mode() string    { return "fabric" }
func (f *Fabric) Actors() []Actor { return append([]Actor{}, f.actors...) }
func (f *Fabric) Close() error {
	for _, gw := range f.gateways {
		gw.Close()
	}
	if f.conn != nil {
		return f.conn.Close()
	}
	return nil
}
func (f *Fabric) Invoke(ctx context.Context, actor, method string, args ...string) (json.RawMessage, error) {
	contract := f.contracts[actor]
	if contract == nil {
		return nil, errors.New("unknown configured Fabric identity")
	}
	// Use the official Gateway SDK to evaluate reads and synchronously submit
	// writes (with final commit status). Each server identity is an enrolled
	// certificate; HTML / JSON cannot grant the dpp.role attribute.
	type result struct {
		raw []byte
		err error
	}
	done := make(chan result, 1)
	go func() {
		var raw []byte
		var err error
		if isRead(method) {
			raw, err = contract.EvaluateTransaction(method, args...)
		} else {
			raw, err = contract.SubmitTransaction(method, args...)
		}
		done <- result{raw, err}
	}()
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case out := <-done:
		if out.err != nil {
			return nil, fmt.Errorf("Fabric Gateway: %w", out.err)
		}
		if !json.Valid(out.raw) {
			return nil, fmt.Errorf("Fabric Gateway returned non-JSON for %s", method)
		}
		return json.RawMessage(out.raw), nil
	}
}
