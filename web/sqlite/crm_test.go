package sqlite

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/gosom/google-maps-scraper/web"
)

func TestOffersAndOpportunitiesPersistIndependently(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "crm.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	crm, ok := repository.(web.CRMRepository)
	if !ok {
		t.Fatal("SQLite repository must implement CRMRepository")
	}
	ctx := context.Background()

	offerA, err := crm.SaveOffer(ctx, web.Offer{Name: "Site institucional customizado", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	offerB, err := crm.SaveOffer(ctx, web.Offer{Name: "Edição avançada de vídeo", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	lead := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	priceA, priceB := int64(125000), int64(39000)

	oppA, err := crm.SaveOpportunity(ctx, web.Opportunity{LeadKey: lead, OfferID: offerA.ID, PriceCents: &priceA, Stage: web.CRMStageProposal, NextAction: "Enviar proposta"})
	if err != nil {
		t.Fatal(err)
	}
	if oppA.UpdatedAt.Nanosecond() != 0 {
		t.Fatalf("save response timestamp should match stored second precision: %s", oppA.UpdatedAt)
	}
	oppB, err := crm.SaveOpportunity(ctx, web.Opportunity{LeadKey: lead, OfferID: offerB.ID, PriceCents: &priceB, Stage: web.CRMStageQualified})
	if err != nil {
		t.Fatal(err)
	}

	got, err := crm.ListOpportunities(ctx, lead)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("expected two independent opportunities, got %d", len(got))
	}
	byID := map[int64]web.Opportunity{}
	for _, opportunity := range got {
		byID[opportunity.ID] = opportunity
	}
	if byID[oppA.ID].UpdatedAt.Nanosecond() != 0 {
		t.Fatalf("created opportunity timestamp should match stored second precision: %s", byID[oppA.ID].UpdatedAt)
	}
	if byID[oppA.ID].OfferID != offerA.ID || byID[oppA.ID].PriceCents == nil || *byID[oppA.ID].PriceCents != priceA || byID[oppA.ID].NextAction != "Enviar proposta" {
		t.Fatalf("unexpected first opportunity: %+v", byID[oppA.ID])
	}
	if byID[oppB.ID].OfferID != offerB.ID || byID[oppB.ID].PriceCents == nil || *byID[oppB.ID].PriceCents != priceB {
		t.Fatalf("unexpected second opportunity: %+v", byID[oppB.ID])
	}

	updatedPrice := int64(150000)
	oppA.PriceCents = &updatedPrice
	oppA.Stage = web.CRMStageWon
	if _, err := crm.SaveOpportunity(ctx, oppA); err != nil {
		t.Fatal(err)
	}
	got, err = crm.ListOpportunities(ctx, lead)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("updating one opportunity must not replace the other: %+v", got)
	}
	for _, opportunity := range got {
		if opportunity.ID == oppA.ID && (opportunity.Stage != web.CRMStageWon || *opportunity.PriceCents != updatedPrice) {
			t.Fatalf("updated opportunity did not persist: %+v", opportunity)
		}
		if opportunity.ID == oppB.ID && *opportunity.PriceCents != priceB {
			t.Fatalf("updating one opportunity changed another: %+v", opportunity)
		}
	}
}

func TestOfferInUseCannotBeDeletedAfterSQLiteConnectionRotation(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "crm.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	store := repository.(web.CRMRepository)
	lead := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	if _, err := store.SaveOpportunity(context.Background(), web.Opportunity{LeadKey: lead, OfferID: 1, Stage: web.CRMStageNew}); err != nil {
		t.Fatal(err)
	}

	// A zero idle limit forces database/sql to open a fresh SQLite connection
	// for the next operation. Connection-local PRAGMAs must still be applied.
	repository.(*repo).db.SetMaxIdleConns(0)
	if err := store.DeleteOffer(context.Background(), 1); err == nil {
		t.Fatal("deleting an offer referenced by an opportunity must be rejected after connection rotation")
	}
}

func TestOfferSeedMarkerRollsBackWithPartialSeedFailure(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "seed.sqlite"))
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
		CREATE TRIGGER fail_one_seed BEFORE INSERT ON crm_offers
		WHEN NEW.name = 'Edição de vídeo'
		BEGIN SELECT RAISE(ABORT, 'test seed failure'); END;
	`)
	if err != nil {
		t.Fatal(err)
	}
	if err := createSchema(db); err == nil {
		t.Fatal("expected injected offer seed failure")
	}
	var markerCount, offerCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM crm_meta WHERE key = 'offer_catalog_v1'`).Scan(&markerCount); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM crm_offers`).Scan(&offerCount); err != nil {
		t.Fatal(err)
	}
	if markerCount != 0 || offerCount != 0 {
		t.Fatalf("partial seed persisted after failure: marker=%d offers=%d", markerCount, offerCount)
	}
	if _, err := db.Exec(`DROP TRIGGER fail_one_seed`); err != nil {
		t.Fatal(err)
	}
	if err := createSchema(db); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM crm_offers`).Scan(&offerCount); err != nil {
		t.Fatal(err)
	}
	if offerCount != 5 {
		t.Fatalf("retry should seed all offers, got %d", offerCount)
	}
}

func TestOfferCatalogCanRepresentUnpricedOffers(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "crm.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	crm := repository.(web.CRMRepository)
	offer, err := crm.SaveOffer(context.Background(), web.Offer{Name: "Fluência prática em IA", Active: true})
	if err != nil {
		t.Fatal(err)
	}
	if offer.DefaultPriceCents != nil {
		t.Fatalf("unspecified price must remain pending, got %v", *offer.DefaultPriceCents)
	}
}
