package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/web"
)

func TestCRMSeededCatalogMatchesPublishedServicesAndCombos(t *testing.T) {
	handler := newCRMTestServer(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crm/offers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET offers status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var offers []web.Offer
	if err := json.Unmarshal(rec.Body.Bytes(), &offers); err != nil {
		t.Fatal(err)
	}

	wantPrices := map[string]int64{
		"Combo Presença Local":     49000,
		"Combo Aceleração Total":   149000,
		"Combo Futuro & Escala IA": 99000,
	}
	wantNames := []string{
		"Placas de Avaliação Google (NFC + QR Code)",
		"Perfil de Empresa no Google",
		"Criação de site ou landing page",
		"Gestão de Google Ads",
		"Influencer / Clone de IA",
		"Combo Presença Local",
		"Combo Aceleração Total",
		"Combo Futuro & Escala IA",
	}
	if len(offers) != len(wantNames) {
		t.Fatalf("expected %d active site-aligned offers, got %d: %+v", len(wantNames), len(offers), offers)
	}
	byName := make(map[string]web.Offer, len(offers))
	for _, offer := range offers {
		byName[offer.Name] = offer
	}
	for _, name := range wantNames {
		if _, ok := byName[name]; !ok {
			t.Errorf("missing active site offer %q", name)
		}
	}
	for name, cents := range wantPrices {
		offer := byName[name]
		if offer.DefaultPriceCents == nil || *offer.DefaultPriceCents != cents {
			t.Errorf("%s default price = %v, want %d cents", name, offer.DefaultPriceCents, cents)
		}
	}
	if !strings.Contains(byName["Combo Aceleração Total"].Description, "R$ 600 a R$ 900/mês") || !strings.Contains(byName["Combo Aceleração Total"].Description, "à parte") {
		t.Errorf("Aceleração Total must separate monthly management and Google ad spend: %q", byName["Combo Aceleração Total"].Description)
	}
}

func TestCRMOpportunityPersistsMonthlyFeeSeparately(t *testing.T) {
	handler := newCRMTestServer(t)
	var offers []web.Offer
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crm/offers", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET offers status = %d, body=%s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &offers); err != nil {
		t.Fatal(err)
	}
	var offerID int64
	for _, offer := range offers {
		if offer.Name == "Combo Aceleração Total" {
			offerID = offer.ID
		}
	}
	if offerID == 0 {
		t.Fatal("Combo Aceleração Total is missing from the active catalog")
	}

	key := strings.Repeat("a", 64)
	body := `{"lead_key":"` + key + `","offer_id":` + strconv.FormatInt(offerID, 10) + `,"price_cents":149000,"monthly_price_cents":60000,"stage":"proposal","notes":"Verba de anúncios paga diretamente ao Google, à parte."}`
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/crm/opportunities", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST opportunity status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var saved map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved["price_cents"] != float64(149000) || saved["monthly_price_cents"] != float64(60000) {
		t.Fatalf("one-time and monthly values were not persisted separately: %#v", saved)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/crm/opportunities?lead_key="+key, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET opportunity status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var persisted []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &persisted); err != nil {
		t.Fatal(err)
	}
	if len(persisted) != 1 || persisted[0]["price_cents"] != float64(149000) || persisted[0]["monthly_price_cents"] != float64(60000) {
		t.Fatalf("one-time and monthly values did not survive reload: %#v", persisted)
	}
}

func TestCRMOpportunityRejectsNegativeMonthlyFee(t *testing.T) {
	handler := newCRMTestServer(t)
	body := `{"lead_key":"` + strings.Repeat("b", 64) + `","offer_id":1,"monthly_price_cents":-1,"stage":"proposal"}`
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/v1/crm/opportunities", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("negative monthly fee should be rejected, got %d: %s", rec.Code, rec.Body.String())
	}
}
