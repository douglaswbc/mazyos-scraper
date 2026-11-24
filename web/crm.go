package web

import (
	"context"
	"time"
)

type CRMStage string

const (
	CRMStageNew         CRMStage = "new"
	CRMStageQualified   CRMStage = "qualified"
	CRMStageProposal    CRMStage = "proposal"
	CRMStageNegotiation CRMStage = "negotiation"
	CRMStageWon         CRMStage = "won"
	CRMStageLost        CRMStage = "lost"
)

func (s CRMStage) Valid() bool {
	switch s {
	case CRMStageNew, CRMStageQualified, CRMStageProposal, CRMStageNegotiation, CRMStageWon, CRMStageLost:
		return true
	default:
		return false
	}
}

type Offer struct {
	ID                int64     `json:"id"`
	Name              string    `json:"name"`
	Description       string    `json:"description"`
	DefaultPriceCents *int64    `json:"default_price_cents,omitempty"`
	Active            bool      `json:"active"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

type Opportunity struct {
	ID                int64      `json:"id"`
	LeadKey           string     `json:"lead_key"`
	OfferID           int64      `json:"offer_id"`
	OfferName         string     `json:"offer_name,omitempty"`
	PriceCents        *int64     `json:"price_cents,omitempty"`
	MonthlyPriceCents *int64     `json:"monthly_price_cents,omitempty"`
	Stage             CRMStage   `json:"stage"`
	NextAction        string     `json:"next_action"`
	NextActionAt      *time.Time `json:"next_action_at,omitempty"`
	Notes             string     `json:"notes"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type CRMRepository interface {
	ListOffers(context.Context, bool) ([]Offer, error)
	SaveOffer(context.Context, Offer) (Offer, error)
	DeleteOffer(context.Context, int64) error
	ListOpportunities(context.Context, string) ([]Opportunity, error)
	SaveOpportunity(context.Context, Opportunity) (Opportunity, error)
	DeleteOpportunity(context.Context, int64) error
}
