package model

type catalog_item struct {
	seller_id   string
	domain      string
	city        string
	provider_id string
	item_id     string
	item_data   string
	price       int
	category_id string
	colour      string
	gender      string
	concern     string
	ingredient  string
	conscious   string
	preference  string
	formulation string
	skin_type   string
	weight      int
	length      int
	breadth     int
	height      int
	material    string
}

type catalog_bpp struct {
	seller_id           string
	domain              string
	city                string
	catalog_fulfillment string
	catalog_descriptor  string
}

type catalog_provider struct {
	seller_id     string
	domain        string
	city          string
	provider_id   string
	provider_data string
}

var CatalogBPPColumns = []string{
	"seller_id", "domain", "city", "catalog_fulfillment", "catalog_descriptor",
}

func (b catalog_bpp) Values() []any {
	return []any{
		b.seller_id,
		b.domain,
		b.city,
		jsonbOrNil(b.catalog_fulfillment),
		jsonbOrNil(b.catalog_descriptor),
	}
}

var CatalogProviderColumns = []string{
	"seller_id", "domain", "city", "provider_id", "provider_data",
}

func (p catalog_provider) Values() []any {
	return []any{
		p.seller_id,
		p.domain,
		p.city,
		p.provider_id,
		jsonbOrNil(p.provider_data),
	}
}

var CatalogItemColumns = []string{
	"seller_id", "domain", "city", "provider_id", "item_id", "item_data",
	"price", "category_id", "colour", "gender", "concern", "ingredient",
	"conscious", "preference", "formulation", "skin_type",
	"weight", "length", "breadth", "height", "material",
}

func (i catalog_item) Values() []any {
	return []any{
		i.seller_id, i.domain, i.city, i.provider_id, i.item_id,
		jsonbOrNil(i.item_data), i.price, i.category_id,
		i.colour, i.gender, i.concern, i.ingredient,
		i.conscious, i.preference, i.formulation, i.skin_type,
		i.weight, i.length, i.breadth, i.height, i.material,
	}
}

// jsonbOrNil guards the JSONB columns: an empty string is not valid JSON and
// would fail the insert, so send SQL NULL instead.
func jsonbOrNil(s string) any {
	if s == "" {
		return nil
	}
	return s
}
