package web_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/web"
	"github.com/gosom/google-maps-scraper/web/sqlite"
)

func newCRMTestServer(t *testing.T) http.Handler {
	t.Helper()
	repo, err := sqlite.New(t.TempDir() + "/crm.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	srv, err := web.New(web.NewService(repo, t.TempDir()), ":0")
	if err != nil {
		t.Fatal(err)
	}
	return srv
}

func TestCRMOffersAPIStoresEditableCatalogAndPendingPrices(t *testing.T) {
	handler := newCRMTestServer(t)
	body := `{"name":"Site institucional","description":"Projeto sob medida","active":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/crm/offers", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("POST offer status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var offer web.Offer
	if err := json.Unmarshal(rec.Body.Bytes(), &offer); err != nil {
		t.Fatal(err)
	}
	if offer.ID == 0 || offer.Name != "Site institucional" || offer.DefaultPriceCents != nil {
		t.Fatalf("unexpected offer: %+v", offer)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/crm/offers", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET offers status = %d, body=%s", rec.Code, rec.Body.String())
	}
	var offers []web.Offer
	if err := json.Unmarshal(rec.Body.Bytes(), &offers); err != nil {
		t.Fatal(err)
	}
	if len(offers) != 6 {
		t.Fatalf("expected five initial offers plus the saved offer, got %d: %+v", len(offers), offers)
	}
	found := false
	for _, item := range offers {
		if item.ID == offer.ID {
			found = true
		}
	}
	if !found {
		t.Fatalf("catalog did not persist new offer: %+v", offers)
	}
}

func TestCRMOffersAPIRejectsNegativePriceAndUnknownFields(t *testing.T) {
	handler := newCRMTestServer(t)
	for _, body := range []string{
		`{"name":"Oferta","default_price_cents":-1,"active":true}`,
		`{"name":"Oferta","active":true,"ignored":1}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/crm/offers", strings.NewReader(body))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("body %s: expected 400, got %d", body, rec.Code)
		}
	}
}

func TestCRMOpportunityAPIStoresOfferPriceAndNextAction(t *testing.T) {
	handler := newCRMTestServer(t)
	offerReq := httptest.NewRequest(http.MethodPost, "/api/v1/crm/offers", strings.NewReader(`{"name":"Placa QR + NFC","active":true}`))
	offerRec := httptest.NewRecorder()
	handler.ServeHTTP(offerRec, offerReq)
	if offerRec.Code != http.StatusCreated {
		t.Fatalf("create offer: status=%d body=%s", offerRec.Code, offerRec.Body.String())
	}
	var offer web.Offer
	if err := json.Unmarshal(offerRec.Body.Bytes(), &offer); err != nil {
		t.Fatal(err)
	}

	key := strings.Repeat("a", 64)
	body := `{"lead_key":"` + key + `","offer_id":` + strconv.FormatInt(offer.ID, 10) + `,"price_cents":49900,"stage":"proposal","next_action":"Enviar orçamento","notes":"Preço negociável"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/crm/opportunities", strings.NewReader(body))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create opportunity: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var opportunity web.Opportunity
	if err := json.Unmarshal(rec.Body.Bytes(), &opportunity); err != nil {
		t.Fatal(err)
	}
	if opportunity.PriceCents == nil || *opportunity.PriceCents != 49900 || opportunity.Stage != web.CRMStageProposal || opportunity.NextAction != "Enviar orçamento" {
		t.Fatalf("unexpected opportunity: %+v", opportunity)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/crm/opportunities?lead_key="+key, nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list opportunities: status=%d body=%s", rec.Code, rec.Body.String())
	}
	var opportunities []web.Opportunity
	if err := json.Unmarshal(rec.Body.Bytes(), &opportunities); err != nil {
		t.Fatal(err)
	}
	if len(opportunities) != 1 || opportunities[0].ID != opportunity.ID {
		t.Fatalf("opportunity was not persisted: %+v", opportunities)
	}
}
