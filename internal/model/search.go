package model

import "encoding/json"

// SearchQuery is the POST /search request body produced by the SynthD traffic
// generator. Every query filters on category_id and carries a recipient URL the
// rebuilt payload is POSTed back to (async callback flow).
type SearchQuery struct {
	QueryID     string                `json:"query_id"`
	QueryType   string                `json:"query_type"` // categorical | mixed | range
	Domain      string                `json:"domain"`
	CategoryID  string                `json:"category_id"`
	Categorical map[string]string     `json:"categorical"`
	Range       map[string]RangeBound `json:"range"`
	Sort        string                `json:"sort"`
	Limit       int                   `json:"limit"`
	Recipient   string                `json:"recipient"`
}

// RangeBound is an inclusive [min, max] filter on a numeric column.
type RangeBound struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// RebuiltPayload mirrors the on_search structure, one per BPP (seller). A single
// search can match items across many sellers, so a callback body is a list of
// these. Context holds only the fields that were persisted on ingest.
type RebuiltPayload struct {
	Context RebuiltContext `json:"context"`
	Message RebuiltMessage `json:"message"`
}

type RebuiltContext struct {
	Domain   string `json:"domain"`
	City     string `json:"city"`
	SellerID string `json:"seller_id"`
}

type RebuiltMessage struct {
	Catalog RebuiltCatalog `json:"catalog"`
}

type RebuiltCatalog struct {
	Descriptor json.RawMessage   `json:"bpp/descriptor"`
	Providers  []RebuiltProvider `json:"bpp/providers"`
}

type RebuiltProvider struct {
	ID           string            `json:"id"`
	Fulfillments json.RawMessage   `json:"fulfillments"`
	Items        []json.RawMessage `json:"items"`
}
