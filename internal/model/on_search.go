package model

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
)

type On_Search struct {
	Context context_envelope `json:"context"`
	Message message_envelope `json:"message"`
}

type context_envelope struct {
	Domain         string `json:"domain"`
	Country        string `json:"country"`
	Action         string `json:"action"`
	City           string `json:"city"`
	Core_version   string `json:"core_version"`
	Bap_id         string `json:"bap_id"`
	Bap_uri        string `json:"bap_uri"`
	Bpp_id         string `json:"bpp_id"`
	Bpp_uri        string `json:"bpp_uri"`
	Transaction_id string `json:"transaction_id"`
	Message_id     string `json:"message_id"`
	Timestamp      string `json:"timestamp"`
}

type message_envelope struct {
	Catalog catalog_envelope `json:"catalog"`
}

type catalog_envelope struct {
	Descriptor json.RawMessage     `json:"bpp/descriptor"`
	Providers  []provider_envelope `json:"bpp/providers"`
}

type descriptor_envelope struct {
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	Short_desc string `json:"short_desc"`
	Long_desc  string `json:"long_desc"`
}

type provider_envelope struct {
	Id           string            `json:"id"`
	Fulfillments json.RawMessage   `json:"fulfillments"`
	Items        []json.RawMessage `json:"items"`
}

type item_envelope struct {
	Id             string              `json:"id"`
	Category_id    string              `json:"category_id"`
	Fulfillment_id int                 `json:"fulfillment_id"`
	Price          int                 `json:"price"`
	Tags           []tag_list_envelope `json:"tags"`
}

type tag_list_envelope struct {
	Code string         `json:"code"`
	List []tag_envelope `json:"list"`
}

type tag_envelope struct {
	Code  string `json:"code"`
	Value string `json:"value"`
}

func (c *On_Search) GetBPP() catalog_bpp {
	bpp := catalog_bpp{
		seller_id:          c.Context.Bap_id,
		domain:             c.Context.Domain,
		city:               c.Context.City,
		catalog_descriptor: string(c.Message.Catalog.Descriptor),
	}
	return bpp
}

func (c *On_Search) SplitProvider() []On_Search {
	providers := make([]On_Search, len(c.Message.Catalog.Providers))
	for index, provider := range c.Message.Catalog.Providers {
		providers[index] = On_Search{
			Context: c.Context,
			Message: message_envelope{
				Catalog: catalog_envelope{
					Descriptor: c.Message.Catalog.Descriptor,
					Providers: []provider_envelope{
						provider,
					},
				},
			},
		}
	}
	return providers
}

func (c *On_Search) GetCatalogProvider() catalog_provider {
	if len(c.Message.Catalog.Providers) > 1 {
		log.Fatalln("More than 1 provider found in provider_list, call splitProvider before getting providers")
	}

	provider := catalog_provider{
		seller_id:     c.Context.Bap_id,
		domain:        c.Context.Domain,
		city:          c.Context.City,
		provider_id:   c.Message.Catalog.Providers[0].Id,
		provider_data: string(c.Message.Catalog.Providers[0].Fulfillments),
	}

	return provider
}

func (c *On_Search) GetCatalogItems() []catalog_item {
	if len(c.Message.Catalog.Providers) > 1 {
		log.Fatalln("More than 1 provider found in provider_list, call splitProvider before getting items")
	}

	provider := c.Message.Catalog.Providers[0]
	items := make([]catalog_item, 0, len(provider.Items))

	for _, raw := range provider.Items {
		var item item_envelope
		if err := json.Unmarshal(raw, &item); err != nil {
			log.Fatalln(err)
		}

		attrs := flattenAttributes(item.Tags)

		items = append(items, catalog_item{
			seller_id:   c.Context.Bap_id,
			domain:      c.Context.Domain,
			city:        c.Context.City,
			provider_id: provider.Id,
			item_id:     item.Id,
			item_data:   string(raw),
			price:       item.Price,
			category_id: item.Category_id,
			colour:      attrs["colour"],
			gender:      attrs["gender"],
			concern:     attrs["concern"],
			ingredient:  attrs["ingredient"],
			conscious:   attrs["conscious"],
			preference:  attrs["preference"],
			formulation: attrs["formulation"],
			skin_type:   attrs["skin_type"],
			material:    attrs["material"],
			weight:      atoiSafe(attrs["weight"]),
			length:      atoiSafe(attrs["length"]),
			breadth:     atoiSafe(attrs["breadth"]),
			height:      atoiSafe(attrs["height"]),
		})
	}
	return items
}

func flattenAttributes(tags []tag_list_envelope) map[string]string {
	attrs := make(map[string]string)
	for _, list := range tags {
		if strings.ToLower(list.Code) != "attribute" {
			continue // only the "attribute" tag list holds sub-category attributes
		}
		for _, tag := range list.List {
			attrs[strings.ToLower(tag.Code)] = tag.Value // lowercased defensively
		}
	}
	return attrs
}

func atoiSafe(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0 // empty or unparseable → 0, excluded by the index predicate
	}
	return n
}
