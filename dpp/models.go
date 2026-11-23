// Package dpp implements deterministic artisan-centred Digital Product Passport
// transaction logic. The Fabric contract API adapter lives in cmd/chaincode.
package dpp

// Caller is derived from Fabric's authenticated client certificate, not from
// a submitted transaction argument.
type Caller struct {
	MSPID  string
	Role   string
	IDHash string // SHA-256 of Fabric client identity; never a raw subject/DN.
}

// TxContext abstracts the subset of the Fabric chaincode stub used by the
// contract. The abstraction makes the *same* transaction logic unit-testable.
type TxContext interface {
	GetState(key string) ([]byte, error)
	PutState(key string, value []byte) error
	GetTxID() string
	EmitEvent(name string, payload []byte) error
	Caller() (Caller, error)
}

type Contract struct{}

type Artisan struct {
	ID              string   `json:"id"`
	PublicAlias     string   `json:"publicAlias,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	Public          bool     `json:"public"`
	RegisteredByMSP string   `json:"registeredByMSP"`
	CreatedTxID     string   `json:"createdTxId"`
}

type MaterialUse struct {
	LotID         string `json:"lotId"`
	QuantityGrams int64  `json:"quantityGrams"`
}

type Transfer struct {
	FromMSP string `json:"fromMSP"`
	ToMSP   string `json:"toMSP"`
	TxID    string `json:"txId"`
}

type MaterialLot struct {
	ID             string        `json:"id"`
	MaterialType   string        `json:"materialType"`
	OriginRegion   string        `json:"originRegion"`
	SupplierRef    string        `json:"supplierRef,omitempty"`
	QuantityGrams  int64         `json:"quantityGrams"`
	UsedGrams      int64         `json:"usedGrams"`
	SourceLots     []MaterialUse `json:"sourceLots,omitempty"`
	EvidenceSHA256 string        `json:"evidenceSha256,omitempty"`
	CreatedByMSP   string        `json:"createdByMSP"`
	CustodianMSP   string        `json:"custodianMSP"`
	Transfers      []Transfer    `json:"transfers,omitempty"`
	CreatedTxID    string        `json:"createdTxId"`
}

type Product struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	Category      string   `json:"category"`
	OwnerMSP      string   `json:"ownerMSP"`
	Status        string   `json:"status"` // draft | published
	ActivityIDs   []string `json:"activityIds"`
	CreatedTxID   string   `json:"createdTxId"`
	PublishedTxID string   `json:"publishedTxId,omitempty"`
}

type Contribution struct {
	ArtisanID string `json:"artisanId"`
	Role      string `json:"role"`
}

type Activity struct {
	ID             string         `json:"id"`
	ProductID      string         `json:"productId"`
	Stage          string         `json:"stage"`
	Technique      string         `json:"technique"`
	LocationCode   string         `json:"locationCode,omitempty"`
	Contributors   []Contribution `json:"contributors"`
	Materials      []MaterialUse  `json:"materials,omitempty"`
	EvidenceSHA256 string         `json:"evidenceSha256,omitempty"`
	RecordedByMSP  string         `json:"recordedByMSP"`
	RecordedTxID   string         `json:"recordedTxId"`
	AuditorMSPs    []string       `json:"auditorMSPs,omitempty"`
}

type Attestation struct {
	ActivityID     string `json:"activityId"`
	AuditorMSP     string `json:"auditorMSP"`
	Verdict        string `json:"verdict"` // supported | disputed
	EvidenceSHA256 string `json:"evidenceSha256"`
	RecordedTxID   string `json:"recordedTxId"`
}

// Public DTOs are deliberately projections. They contain no Fabric client
// identity, non-public artisan alias, or source document contents.
type PublicArtisan struct {
	ID    string `json:"id"`
	Alias string `json:"alias,omitempty"`
	Role  string `json:"role"`
}

type PublicActivity struct {
	ID             string          `json:"id"`
	Stage          string          `json:"stage"`
	Technique      string          `json:"technique"`
	LocationCode   string          `json:"locationCode,omitempty"`
	EvidenceSHA256 string          `json:"evidenceSha256,omitempty"`
	Artisans       []PublicArtisan `json:"artisans"`
	Materials      []MaterialUse   `json:"materials,omitempty"`
	Attestations   []Attestation   `json:"attestations,omitempty"`
}

type PublicMaterial struct {
	ID             string        `json:"id"`
	MaterialType   string        `json:"materialType"`
	OriginRegion   string        `json:"originRegion"`
	SourceLots     []MaterialUse `json:"sourceLots,omitempty"`
	EvidenceSHA256 string        `json:"evidenceSha256,omitempty"`
}

type Passport struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Category   string           `json:"category"`
	Status     string           `json:"status"`
	Activities []PublicActivity `json:"activities"`
	Materials  []PublicMaterial `json:"materials"`
}

type ArtisanInput struct {
	ID          string   `json:"id"`
	PublicAlias string   `json:"publicAlias"`
	Skills      []string `json:"skills"`
	Public      bool     `json:"public"`
}

type MaterialInput struct {
	ID             string `json:"id"`
	MaterialType   string `json:"materialType"`
	OriginRegion   string `json:"originRegion"`
	SupplierRef    string `json:"supplierRef"`
	QuantityGrams  int64  `json:"quantityGrams"`
	EvidenceSHA256 string `json:"evidenceSha256"`
}

type DerivedLotInput struct {
	ID             string        `json:"id"`
	MaterialType   string        `json:"materialType"`
	QuantityGrams  int64         `json:"quantityGrams"`
	SourceLots     []MaterialUse `json:"sourceLots"`
	EvidenceSHA256 string        `json:"evidenceSha256"`
}

type ProductInput struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Category string `json:"category"`
}

type ActivityInput struct {
	ID             string         `json:"id"`
	ProductID      string         `json:"productId"`
	Stage          string         `json:"stage"`
	Technique      string         `json:"technique"`
	LocationCode   string         `json:"locationCode"`
	Contributors   []Contribution `json:"contributors"`
	Materials      []MaterialUse  `json:"materials"`
	EvidenceSHA256 string         `json:"evidenceSha256"`
}

type AttestationInput struct {
	ActivityID     string `json:"activityId"`
	Verdict        string `json:"verdict"`
	EvidenceSHA256 string `json:"evidenceSha256"`
}
