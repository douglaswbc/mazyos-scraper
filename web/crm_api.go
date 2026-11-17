package web

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type offerRequest struct {
	Name              string `json:"name"`
	Description       string `json:"description"`
	DefaultPriceCents *int64 `json:"default_price_cents"`
	Active            bool   `json:"active"`
}

type opportunityRequest struct {
	LeadKey      string     `json:"lead_key"`
	OfferID      int64      `json:"offer_id"`
	PriceCents   *int64     `json:"price_cents"`
	Stage        CRMStage   `json:"stage"`
	NextAction   string     `json:"next_action"`
	NextActionAt *time.Time `json:"next_action_at"`
	Notes        string     `json:"notes"`
}

func (s *Server) apiCRMOffers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		offers, err := s.svc.CRMOffers(r.Context(), r.URL.Query().Get("include_inactive") == "true")
		if err != nil {
			renderJSON(w, http.StatusInternalServerError, apiError{Code: 500, Message: "Could not load offers"})
			return
		}
		renderJSON(w, http.StatusOK, offers)
	case http.MethodPost:
		var req offerRequest
		if decodeCRMJSON(w, r, &req) != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid offer"})
			return
		}
		offer, err := s.svc.SaveCRMOffer(r.Context(), Offer{Name: req.Name, Description: req.Description, DefaultPriceCents: req.DefaultPriceCents, Active: req.Active})
		if err != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid offer"})
			return
		}
		renderJSON(w, http.StatusCreated, offer)
	default:
		renderJSON(w, http.StatusMethodNotAllowed, apiError{Code: 405, Message: methodNotAllowedMessage})
	}
}

func (s *Server) apiCRMOffer(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid offer ID"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req offerRequest
		if decodeCRMJSON(w, r, &req) != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid offer"})
			return
		}
		offer, err := s.svc.SaveCRMOffer(r.Context(), Offer{ID: id, Name: req.Name, Description: req.Description, DefaultPriceCents: req.DefaultPriceCents, Active: req.Active})
		if errors.Is(err, sql.ErrNoRows) {
			renderJSON(w, http.StatusNotFound, apiError{Code: 404, Message: "Offer not found"})
			return
		}
		if err != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid offer"})
			return
		}
		renderJSON(w, http.StatusOK, offer)
	case http.MethodDelete:
		err := s.svc.DeleteCRMOffer(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			renderJSON(w, http.StatusNotFound, apiError{Code: 404, Message: "Offer not found"})
			return
		}
		if err != nil {
			renderJSON(w, http.StatusConflict, apiError{Code: 409, Message: "Offer is in use or could not be deleted"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		renderJSON(w, http.StatusMethodNotAllowed, apiError{Code: 405, Message: methodNotAllowedMessage})
	}
}

func (s *Server) apiCRMOpportunities(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		key := r.URL.Query().Get("lead_key")
		if key != "" && !ValidProspectKey(key) {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid lead key"})
			return
		}
		items, err := s.svc.CRMOpportunities(r.Context(), key)
		if err != nil {
			renderJSON(w, http.StatusInternalServerError, apiError{Code: 500, Message: "Could not load opportunities"})
			return
		}
		renderJSON(w, http.StatusOK, items)
	case http.MethodPost:
		var req opportunityRequest
		if decodeCRMJSON(w, r, &req) != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid opportunity"})
			return
		}
		item, err := s.svc.SaveCRMOpportunity(r.Context(), Opportunity{LeadKey: req.LeadKey, OfferID: req.OfferID, PriceCents: req.PriceCents, Stage: req.Stage, NextAction: req.NextAction, NextActionAt: req.NextActionAt, Notes: req.Notes})
		if err != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid opportunity or offer"})
			return
		}
		renderJSON(w, http.StatusCreated, item)
	default:
		renderJSON(w, http.StatusMethodNotAllowed, apiError{Code: 405, Message: methodNotAllowedMessage})
	}
}

func (s *Server) apiCRMOpportunity(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id <= 0 {
		renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid opportunity ID"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var req opportunityRequest
		if decodeCRMJSON(w, r, &req) != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid opportunity"})
			return
		}
		item, err := s.svc.SaveCRMOpportunity(r.Context(), Opportunity{ID: id, LeadKey: req.LeadKey, OfferID: req.OfferID, PriceCents: req.PriceCents, Stage: req.Stage, NextAction: req.NextAction, NextActionAt: req.NextActionAt, Notes: req.Notes})
		if errors.Is(err, sql.ErrNoRows) {
			renderJSON(w, http.StatusNotFound, apiError{Code: 404, Message: "Opportunity not found"})
			return
		}
		if err != nil {
			renderJSON(w, http.StatusBadRequest, apiError{Code: 400, Message: "Invalid opportunity or offer"})
			return
		}
		renderJSON(w, http.StatusOK, item)
	case http.MethodDelete:
		err := s.svc.DeleteCRMOpportunity(r.Context(), id)
		if errors.Is(err, sql.ErrNoRows) {
			renderJSON(w, http.StatusNotFound, apiError{Code: 404, Message: "Opportunity not found"})
			return
		}
		if err != nil {
			renderJSON(w, http.StatusInternalServerError, apiError{Code: 500, Message: "Could not delete opportunity"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		renderJSON(w, http.StatusMethodNotAllowed, apiError{Code: 405, Message: methodNotAllowedMessage})
	}
}

func decodeCRMJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 16*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	if contentType := strings.TrimSpace(r.Header.Get("Content-Type")); contentType != "" && !strings.HasPrefix(contentType, "application/json") {
		return errors.New("invalid content type")
	}
	return nil
}
