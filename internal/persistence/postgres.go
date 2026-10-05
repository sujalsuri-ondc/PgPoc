package persistence

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strings"

	"PgPoc/internal/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var partitions []string = []string{
	"Fragrance",
	"Bath Soaps and Gels",
	"Hair Oils, Care, and Styling",
	"Shampoos and Conditioners",
	"Shaving and Grooming",
	"Beard Care and Tools",
	"Grooming Tools and Accessories",
	"Makeup - Nail Care",
	"Makeup - Eyes",
	"Makeup - Face",
	"Makeup - Lips",
	"Makeup - Body",
	"Makeup - Remover",
	"Makeup - Sets and Kits",
	"Makeup - Tools and Brushes",
	"Makeup - Kits and Combos",
	"Skin Care - Face Cleansers",
	"Skin Care - Hand and Feet",
	"Body Care - Cleansers",
	"Body Care - Moisturizers",
	"Body Care - Loofah and Other Tools",
	"Body Care - Bath Salt and Additives",
	"Hair Care - Shampoo, Oils, Conditioners",
	"Skin Care - Lotions, Moisturisers, and Creams",
	"Skin Care - Oils and Serums",
	"Trimmer",
	"Shaver",
	"Epilator",
	"Hair Straightener",
	"Hair Dryer",
	"Hair Curler",
	"Hair Crimper",
	"Hair Care - Colour",
	"Bath & Body",
	"Feminine Care",
	"Hair Care",
	"Make Up",
	"Mens Grooming",
	"Oral Care",
	"Skin Care",
	"Maternity Care",
	"Nursing & Feeding",
	"Sexual Wellness & Sensuality",
	"Tools & Accessories",
	"Home Decor",
	"Furniture",
	"Home Furnishing - Bedding and Linen",
	"Cleaning Supplies",
	"Bins and Bathroom",
	"Car and Shoe Care",
	"Disposables and Garbage Bags",
	"Fresheners and Repellents",
	"Mops, Brushes and Scrubs",
	"Party and Festive Needs",
	"Flowers",
	"Pooja Needs",
	"Paint and Wallpapers",
	"Hardware, Tools and Home Safety",
	"Electricals",
	"Bathroom and Kitchen fixtures",
	"Garden & Outdoor",
	"Sports and Fitness Equipment",
	"Cookware",
	"Serveware",
	"Kitchen Storage and Containers",
	"Kitchen Tools",
	"Closet/Laundry/Shoe Organization",
	"Toys and Games",
	"Stationery",
	"Books",
	"Gift Voucher",
	"Home Furnishings",
	"Garden and Outdoor Products",
	"Home Improvement",
	"Cookware and Dining",
	"Storage and Organisation",
}

func NewPostgresClient(dsn string) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), dsn)

	if err != nil {
		log.Fatalln(err)
	}

	if err := pool.Ping(context.Background()); err != nil {
		log.Fatalln(err)
	}

	initPostgres(pool)
	return pool
}

func initPostgres(pool *pgxpool.Pool) {
	createCatalogBPP := `CREATE TABLE IF NOT EXISTS catalog_bpp (
		seller_id UUID,
		domain VARCHAR(8),
		city VARCHAR(11),
		catalog_fulfillment JSONB,
		catalog_descriptor JSONB,
		PRIMARY KEY (seller_id, domain, city)
	);`

	if _, err := pool.Exec(context.Background(), createCatalogBPP); err != nil {
		log.Fatalln(err)
	}

	log.Println("Sucessfully initialized catalog_BPP")
	createCatalogProviders := `CREATE TABLE IF NOT EXISTS catalog_providers (
		seller_id UUID,
		domain VARCHAR(8), 
		city VARCHAR(11),
		provider_id UUID,
		provider_data JSONB,
		PRIMARY KEY (seller_id, domain, city, provider_id),
		CONSTRAINT foregin_key_bpp 
			FOREIGN KEY (seller_id, domain, city) 
			REFERENCES catalog_bpp(seller_id, domain, city)
			ON DELETE CASCADE
	);`

	if _, err := pool.Exec(context.Background(), createCatalogProviders); err != nil {
		log.Fatalln(err)
	}

	log.Println("Sucessfully initialized catalog_providers")

	createCatalogItems := `
	CREATE TABLE IF NOT EXISTS catalog_items (
		seller_id UUID,
		domain VARCHAR(8),
		city VARCHAR(11),
		provider_id UUID,
		item_id UUID,
		item_data JSONB,
		price BIGINT,
		category_id VARCHAR(64),
		colour VARCHAR(32),
		gender VARCHAR(8),
		concern VARCHAR(32),
		ingredient VARCHAR(32),
		conscious VARCHAR(32),
		preference VARCHAR(64),
		formulation VARCHAR(32),
		skin_type VARCHAR(16),
		weight BIGINT,
		length BIGINT,
		breadth BIGINT,
		height BIGINT,
		material VARCHAR(32),
		PRIMARY KEY (seller_id, domain, city, provider_id, item_id, category_id),
		CONSTRAINT foregin_key_provider 
			FOREIGN KEY (seller_id, domain, city, provider_id)
			REFERENCES catalog_providers(seller_id, domain, city, provider_id)
			ON DELETE CASCADE
	)
	PARTITION BY LIST (category_id);
	`

	if _, err := pool.Exec(context.Background(), createCatalogItems); err != nil {
		log.Fatalln(err)
	}

	log.Println("Sucessfully initialized catalog_items, with the paritition created")

	createCatalogIndex := `
	CREATE INDEX IF NOT EXISTS price_index ON catalog_items (price) WHERE price IS NOT NULL AND price <> 0;
	CREATE INDEX IF NOT EXISTS weight_index ON catalog_items (weight) WHERE weight IS NOT NULL AND weight <> 0;
	CREATE INDEX IF NOT EXISTS length_index ON catalog_items (length) WHERE length IS NOT NULL AND length <> 0;
	CREATE INDEX IF NOT EXISTS breadth_index ON catalog_items (breadth) WHERE breadth IS NOT NULL AND breadth <> 0;
	CREATE INDEX IF NOT EXISTS height_index ON catalog_items (height) WHERE height IS NOT NULL AND height <> 0;
	CREATE INDEX IF NOT EXISTS colour_index ON catalog_items (colour) WHERE colour IS NOT NULL AND colour <> '';
	CREATE INDEX IF NOT EXISTS gender_index ON catalog_items (gender) WHERE gender IS NOT NULL AND gender <> '';
	CREATE INDEX IF NOT EXISTS concern_index ON catalog_items (concern) WHERE concern IS NOT NULL AND concern <> '';
	CREATE INDEX IF NOT EXISTS ingredient_index ON catalog_items (ingredient) WHERE ingredient IS NOT NULL AND ingredient <> '';
	CREATE INDEX IF NOT EXISTS conscious_index ON catalog_items (conscious) WHERE conscious IS NOT NULL AND conscious <> '';
	CREATE INDEX IF NOT EXISTS preference_index ON catalog_items (preference) WHERE preference IS NOT NULL AND preference <> '';
	CREATE INDEX IF NOT EXISTS formulation_index ON catalog_items (formulation) WHERE formulation IS NOT NULL AND formulation <> '';
	CREATE INDEX IF NOT EXISTS skin_type_index ON catalog_items (skin_type) WHERE skin_type IS NOT NULL AND skin_type <> '';
	CREATE INDEX IF NOT EXISTS category_id_index ON catalog_items (category_id) WHERE category_id IS NOT NULL AND category_id <> '';
	CREATE INDEX IF NOT EXISTS material_index ON catalog_items (material) WHERE material IS NOT NULL AND material <> '';
	`

	if _, err := pool.Exec(context.Background(), createCatalogIndex); err != nil {
		log.Fatalln(err)
	}

	log.Println("Sucessfully created indexes on all categories")

	for _, parition_name := range partitions {
		childTable := fmt.Sprintf("CREATE TABLE IF NOT EXISTS catalog_items_%v PARTITION OF catalog_items FOR VALUES IN ('%v')", sanitize(parition_name), parition_name)
		if _, err := pool.Exec(context.Background(), childTable); err != nil {
			log.Fatalln(err)
		}
	}

	log.Println("All The parition are successfully created")

	defaultPartition := `CREATE TABLE IF NOT EXISTS catalog_items_default PARTITION OF catalog_items DEFAULT;`
	if _, err := pool.Exec(context.Background(), defaultPartition); err != nil {
		log.Fatalln(err)
	}
	log.Println("Successfully created the default partition")
}

func sanitize(s string) string {
	s = strings.ToLower(s)
	return regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(s, "_")
}

func IngestOnSearch(ctx context.Context, pool *pgxpool.Pool, payload model.On_Search) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // no-op once Commit succeeds

	for _, single := range payload.SplitProvider() {
		// bpp: repeats across the provider split, so ON CONFLICT DO NOTHING.
		bpp := single.GetBPP()
		if err := insertRow(ctx, tx, "catalog_bpp", model.CatalogBPPColumns,
			bpp.Values(), "(seller_id, domain, city)"); err != nil {
			return err
		}

		// provider: one per split payload.
		provider := single.GetCatalogProvider()
		if err := insertRow(ctx, tx, "catalog_providers", model.CatalogProviderColumns,
			provider.Values(), "(seller_id, domain, city, provider_id)"); err != nil {
			return err
		}

		// items: the bulk → CopyFrom (unique ids, no conflicts).
		items := single.GetCatalogItems()
		if len(items) > 0 {
			rows := make([][]any, len(items))
			for i, it := range items {
				rows[i] = it.Values()
			}
			if _, err := tx.CopyFrom(ctx, pgx.Identifier{"catalog_items"},
				model.CatalogItemColumns, pgx.CopyFromRows(rows)); err != nil {
				return fmt.Errorf("copy items: %w", err)
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// insertRow does a single parameterized INSERT ... ON CONFLICT <target> DO NOTHING.
// Generic so it works with any table's exported Columns + Values(), avoiding the
// unexported row-struct types in signatures.
func insertRow(ctx context.Context, tx pgx.Tx, table string, columns []string, values []any, conflictTarget string) error {
	placeholders := make([]string, len(columns))
	for i := range columns {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	sql := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) ON CONFLICT %s DO NOTHING",
		table,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		conflictTarget,
	)
	if _, err := tx.Exec(ctx, sql, values...); err != nil {
		return fmt.Errorf("insert into %s: %w", table, err)
	}
	return nil
}
