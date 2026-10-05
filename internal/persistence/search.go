package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"PgPoc/internal/model"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Whitelisted filter columns. Column names cannot be parameterized, so every
// key coming from the request is validated against these sets before it is
// interpolated into SQL. Anything not listed is rejected.
var allowedCategorical = map[string]bool{
	"colour": true, "gender": true, "concern": true, "ingredient": true,
	"conscious": true, "preference": true, "formulation": true,
	"skin_type": true, "material": true,
}

var allowedRange = map[string]bool{
	"price": true, "weight": true, "length": true, "breadth": true, "height": true,
}

// RunSearch translates the SearchQuery into SQL, runs it, and groups the joined
// rows into one RebuiltPayload per BPP (seller). It returns the payloads and the
// total number of matched item rows.
func RunSearch(ctx context.Context, pool *pgxpool.Pool, q model.SearchQuery) ([]model.RebuiltPayload, int, error) {
	sqlText, args, err := buildSearchSQL(q)
	if err != nil {
		return nil, 0, err
	}

	rows, err := pool.Query(ctx, sqlText, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	// Group deterministically: preserve first-seen order of sellers and providers
	// (rows arrive ordered by price).
	type providerAcc struct {
		fulfillments json.RawMessage
		items        []json.RawMessage
	}
	type bppAcc struct {
		domain       string
		city         string
		sellerID     string
		descriptor   json.RawMessage
		providerKeys []string
		providers    map[string]*providerAcc
	}

	bppOrder := []string{}
	bpps := map[string]*bppAcc{}
	rowCount := 0

	for rows.Next() {
		var sellerID, domain, city, providerID string
		var itemData, providerData, descriptor []byte
		if err := rows.Scan(&sellerID, &domain, &city, &providerID, &itemData, &providerData, &descriptor); err != nil {
			return nil, 0, fmt.Errorf("scan search row: %w", err)
		}
		rowCount++

		bppKey := sellerID + "|" + domain + "|" + city
		bpp, ok := bpps[bppKey]
		if !ok {
			bpp = &bppAcc{
				domain: domain, city: city, sellerID: sellerID,
				descriptor: json.RawMessage(descriptor),
				providers:  map[string]*providerAcc{},
			}
			bpps[bppKey] = bpp
			bppOrder = append(bppOrder, bppKey)
		}

		prov, ok := bpp.providers[providerID]
		if !ok {
			prov = &providerAcc{fulfillments: json.RawMessage(providerData)}
			bpp.providers[providerID] = prov
			bpp.providerKeys = append(bpp.providerKeys, providerID)
		}
		prov.items = append(prov.items, json.RawMessage(itemData))
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate search rows: %w", err)
	}

	payloads := make([]model.RebuiltPayload, 0, len(bppOrder))
	for _, bppKey := range bppOrder {
		bpp := bpps[bppKey]
		providers := make([]model.RebuiltProvider, 0, len(bpp.providerKeys))
		for _, pk := range bpp.providerKeys {
			prov := bpp.providers[pk]
			providers = append(providers, model.RebuiltProvider{
				ID:           pk,
				Fulfillments: prov.fulfillments,
				Items:        prov.items,
			})
		}
		payloads = append(payloads, model.RebuiltPayload{
			Context: model.RebuiltContext{
				Domain: bpp.domain, City: bpp.city, SellerID: bpp.sellerID,
			},
			Message: model.RebuiltMessage{
				Catalog: model.RebuiltCatalog{
					Descriptor: bpp.descriptor,
					Providers:  providers,
				},
			},
		})
	}
	return payloads, rowCount, nil
}

// buildSearchSQL constructs a parameterized SELECT that first filters+limits the
// items (partition-pruned by category_id, ordered by price) and then joins to
// providers and bpp so every row carries the data needed to rebuild the payload.
func buildSearchSQL(q model.SearchQuery) (string, []any, error) {
	if strings.TrimSpace(q.CategoryID) == "" {
		return "", nil, fmt.Errorf("category_id is required")
	}
	if q.Limit <= 0 {
		return "", nil, fmt.Errorf("limit must be positive, got %d", q.Limit)
	}

	args := []any{q.CategoryID}
	conds := []string{"category_id = $1"}

	// Deterministic column order for stable SQL (and stable tests/plans).
	catCols := sortedKeys(q.Categorical)
	for _, col := range catCols {
		if !allowedCategorical[col] {
			return "", nil, fmt.Errorf("unknown categorical filter column %q", col)
		}
		args = append(args, q.Categorical[col])
		conds = append(conds, fmt.Sprintf("%s = $%d", col, len(args)))
	}

	rangeCols := sortedRangeKeys(q.Range)
	for _, col := range rangeCols {
		if !allowedRange[col] {
			return "", nil, fmt.Errorf("unknown range filter column %q", col)
		}
		b := q.Range[col]
		args = append(args, b.Min)
		lo := len(args)
		args = append(args, b.Max)
		hi := len(args)
		conds = append(conds, fmt.Sprintf("%s BETWEEN $%d AND $%d", col, lo, hi))
	}

	args = append(args, q.Limit)
	limitPlaceholder := len(args)

	sub := fmt.Sprintf(`SELECT seller_id, domain, city, provider_id, item_data, price
FROM catalog_items
WHERE %s
ORDER BY price
LIMIT $%d`, strings.Join(conds, " AND "), limitPlaceholder)

	full := fmt.Sprintf(`SELECT i.seller_id, i.domain, i.city, i.provider_id, i.item_data, p.provider_data, b.catalog_descriptor
FROM (%s) i
JOIN catalog_providers p
  ON p.seller_id = i.seller_id AND p.domain = i.domain AND p.city = i.city AND p.provider_id = i.provider_id
JOIN catalog_bpp b
  ON b.seller_id = i.seller_id AND b.domain = i.domain AND b.city = i.city
ORDER BY i.price`, sub)

	return full, args, nil
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func sortedRangeKeys(m map[string]model.RangeBound) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
