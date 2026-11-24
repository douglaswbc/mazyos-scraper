package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gosom/google-maps-scraper/web"
)

func TestExistingCRMDatabaseMigratesMonthlyPriceColumn(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "existing-crm.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE crm_offers (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '', default_price_cents INTEGER,
			active INTEGER NOT NULL DEFAULT 1, created_at INT NOT NULL, updated_at INT NOT NULL
		);
		CREATE TABLE crm_opportunities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			lead_key TEXT NOT NULL,
			offer_id INTEGER NOT NULL REFERENCES crm_offers(id) ON DELETE RESTRICT,
			price_cents INTEGER,
			stage TEXT NOT NULL,
			next_action TEXT NOT NULL DEFAULT '',
			next_action_at INT,
			notes TEXT NOT NULL DEFAULT '',
			created_at INT NOT NULL,
			updated_at INT NOT NULL
		);
		CREATE TABLE crm_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO crm_meta (key, value) VALUES ('offer_catalog_v1', 'seeded');
		INSERT INTO crm_offers (name, description, default_price_cents, active, created_at, updated_at)
		VALUES ('Legacy custom', 'Existing description', 175000, 1, 1, 1);
		INSERT INTO crm_opportunities (lead_key, offer_id, price_cents, stage, created_at, updated_at)
		VALUES ('0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef',
			(SELECT id FROM crm_offers WHERE name = 'Legacy custom'), 149000, 'proposal', 1, 1);
	`)
	if err != nil {
		t.Fatal(err)
	}

	if err := createSchema(db); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`PRAGMA table_info(crm_opportunities)`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var cid, notnull, pk int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&cid, &name, &dataType, &notnull, &defaultValue, &pk); err != nil {
			t.Fatal(err)
		}
		if name == "monthly_price_cents" {
			found = true
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("existing CRM database is missing the separate monthly price column")
	}
	var oneTime int64
	var monthly sql.NullInt64
	if err := db.QueryRow(`SELECT price_cents, monthly_price_cents FROM crm_opportunities`).Scan(&oneTime, &monthly); err != nil {
		t.Fatal(err)
	}
	if oneTime != 149000 || monthly.Valid {
		t.Fatalf("migration should preserve the existing one-time price and leave monthly fee unset: one-time=%d monthly=%v", oneTime, monthly)
	}
}

func TestSiteCatalogUpgradePreservesCustomizedLegacyOffers(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "custom-offer.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE crm_offers (
			id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '', default_price_cents INTEGER,
			active INTEGER NOT NULL DEFAULT 1, created_at INT NOT NULL, updated_at INT NOT NULL
		);
		CREATE TABLE crm_meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
		INSERT INTO crm_meta (key, value) VALUES ('offer_catalog_v1', 'seeded');
		INSERT INTO crm_offers (name, description, default_price_cents, active, created_at, updated_at)
		VALUES ('Criação de site', 'Descrição personalizada', 275000, 1, 1, 1),
		       ('Edição de vídeo', '', NULL, 1, 1, 1);
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := createSchema(db); err != nil {
		t.Fatal(err)
	}
	var customActive, customPrice, legacyActive int
	var customDescription string
	if err := db.QueryRow(`SELECT active, description, default_price_cents FROM crm_offers WHERE name = 'Criação de site'`).Scan(&customActive, &customDescription, &customPrice); err != nil {
		t.Fatal(err)
	}
	if customActive != 1 || customDescription != "Descrição personalizada" || customPrice != 275000 {
		t.Fatalf("customized legacy offer must be preserved: active=%d description=%q price=%d", customActive, customDescription, customPrice)
	}
	if err := db.QueryRow(`SELECT active FROM crm_offers WHERE name = 'Edição de vídeo'`).Scan(&legacyActive); err != nil {
		t.Fatal(err)
	}
	if legacyActive != 0 {
		t.Fatalf("unmodified legacy offer should be archived, active=%d", legacyActive)
	}
}

func TestUnspecifiedOpportunityPricesRemainPending(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "pending-prices.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	crm := repository.(web.CRMRepository)
	offer, err := crm.SaveOffer(context.Background(), web.Offer{Name: "Preço sob consulta", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	lead := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	saved, err := crm.SaveOpportunity(context.Background(), web.Opportunity{LeadKey: lead, OfferID: offer.ID, Stage: web.CRMStageNew})
	if err != nil {
		t.Fatal(err)
	}
	if saved.PriceCents != nil || saved.MonthlyPriceCents != nil {
		t.Fatalf("unspecified prices should remain pending, got one-time=%v monthly=%v", saved.PriceCents, saved.MonthlyPriceCents)
	}
	loaded, err := crm.ListOpportunities(context.Background(), lead)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1 || loaded[0].PriceCents != nil || loaded[0].MonthlyPriceCents != nil {
		t.Fatalf("unspecified prices did not remain pending after reload: %+v", loaded)
	}
}
