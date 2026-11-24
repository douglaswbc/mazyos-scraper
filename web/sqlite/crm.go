package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gosom/google-maps-scraper/web"
)

func (repo *repo) ListOffers(ctx context.Context, includeInactive bool) ([]web.Offer, error) {
	q := `SELECT id, name, description, default_price_cents, active, created_at, updated_at FROM crm_offers`
	if !includeInactive {
		q += ` WHERE active = 1`
	}
	q += ` ORDER BY active DESC, name COLLATE NOCASE, id`
	rows, err := repo.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ans := []web.Offer{}
	for rows.Next() {
		var offer web.Offer
		var price sql.NullInt64
		var active int
		var created, updated int64
		if err := rows.Scan(&offer.ID, &offer.Name, &offer.Description, &price, &active, &created, &updated); err != nil {
			return nil, err
		}
		if price.Valid {
			value := price.Int64
			offer.DefaultPriceCents = &value
		}
		offer.Active = active != 0
		offer.CreatedAt = time.Unix(created, 0).UTC()
		offer.UpdatedAt = time.Unix(updated, 0).UTC()
		ans = append(ans, offer)
	}
	return ans, rows.Err()
}

func (repo *repo) SaveOffer(ctx context.Context, offer web.Offer) (web.Offer, error) {
	offer.Name = strings.TrimSpace(offer.Name)
	offer.Description = strings.TrimSpace(offer.Description)
	if offer.Name == "" || len(offer.Name) > 120 || len(offer.Description) > 1000 || (offer.DefaultPriceCents != nil && *offer.DefaultPriceCents < 0) {
		return web.Offer{}, errors.New("invalid offer")
	}
	now := time.Now().UTC().Unix()
	var price any
	if offer.DefaultPriceCents != nil {
		price = *offer.DefaultPriceCents
	}
	if offer.ID == 0 {
		result, err := repo.db.ExecContext(ctx, `INSERT INTO crm_offers (name, description, default_price_cents, active, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, offer.Name, offer.Description, price, offer.Active, now, now)
		if err != nil {
			return web.Offer{}, err
		}
		offer.ID, err = result.LastInsertId()
		if err != nil {
			return web.Offer{}, err
		}
		offer.CreatedAt = time.Unix(now, 0).UTC()
	} else {
		result, err := repo.db.ExecContext(ctx, `UPDATE crm_offers SET name = ?, description = ?, default_price_cents = ?, active = ?, updated_at = ? WHERE id = ?`, offer.Name, offer.Description, price, offer.Active, now, offer.ID)
		if err != nil {
			return web.Offer{}, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return web.Offer{}, err
		}
		if count == 0 {
			return web.Offer{}, sql.ErrNoRows
		}
		var created int64
		if err := repo.db.QueryRowContext(ctx, `SELECT created_at FROM crm_offers WHERE id = ?`, offer.ID).Scan(&created); err != nil {
			return web.Offer{}, err
		}
		offer.CreatedAt = time.Unix(created, 0).UTC()
	}
	offer.UpdatedAt = time.Unix(now, 0).UTC()
	return offer, nil
}

func (repo *repo) DeleteOffer(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid offer id")
	}
	result, err := repo.db.ExecContext(ctx, `DELETE FROM crm_offers WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (repo *repo) ListOpportunities(ctx context.Context, leadKey string) ([]web.Opportunity, error) {
	if leadKey != "" && !web.ValidProspectKey(leadKey) {
		return nil, errors.New("invalid lead key")
	}
	q := `SELECT o.id, o.lead_key, o.offer_id, f.name, o.price_cents, o.monthly_price_cents, o.stage, o.next_action, o.next_action_at, o.notes, o.created_at, o.updated_at FROM crm_opportunities o JOIN crm_offers f ON f.id = o.offer_id`
	args := []any{}
	if leadKey != "" {
		q += ` WHERE o.lead_key = ?`
		args = append(args, leadKey)
	}
	q += ` ORDER BY CASE WHEN o.next_action_at IS NULL THEN 1 ELSE 0 END, o.next_action_at, o.updated_at DESC, o.id DESC`
	rows, err := repo.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ans := []web.Opportunity{}
	for rows.Next() {
		item, err := scanOpportunity(rows)
		if err != nil {
			return nil, err
		}
		ans = append(ans, item)
	}
	return ans, rows.Err()
}

func (repo *repo) SaveOpportunity(ctx context.Context, item web.Opportunity) (web.Opportunity, error) {
	if !web.ValidProspectKey(item.LeadKey) || item.OfferID <= 0 || !item.Stage.Valid() || len(item.NextAction) > 500 || len(item.Notes) > 4000 || (item.PriceCents != nil && *item.PriceCents < 0) || (item.MonthlyPriceCents != nil && *item.MonthlyPriceCents < 0) {
		return web.Opportunity{}, errors.New("invalid opportunity")
	}
	var nextAt any
	if item.NextActionAt != nil {
		nextAt = item.NextActionAt.UTC().Unix()
	}
	var price any
	if item.PriceCents != nil {
		price = *item.PriceCents
	}
	var monthlyPrice any
	if item.MonthlyPriceCents != nil {
		monthlyPrice = *item.MonthlyPriceCents
	}
	now := time.Now().UTC().Unix()
	updatedAt := time.Unix(now, 0).UTC()
	if item.ID == 0 {
		result, err := repo.db.ExecContext(ctx, `INSERT INTO crm_opportunities (lead_key, offer_id, price_cents, monthly_price_cents, stage, next_action, next_action_at, notes, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, item.LeadKey, item.OfferID, price, monthlyPrice, string(item.Stage), strings.TrimSpace(item.NextAction), nextAt, strings.TrimSpace(item.Notes), now, now)
		if err != nil {
			return web.Opportunity{}, err
		}
		item.ID, err = result.LastInsertId()
		if err != nil {
			return web.Opportunity{}, err
		}
		item.CreatedAt = time.Unix(now, 0).UTC()
	} else {
		result, err := repo.db.ExecContext(ctx, `UPDATE crm_opportunities SET lead_key = ?, offer_id = ?, price_cents = ?, monthly_price_cents = ?, stage = ?, next_action = ?, next_action_at = ?, notes = ?, updated_at = ? WHERE id = ?`, item.LeadKey, item.OfferID, price, monthlyPrice, string(item.Stage), strings.TrimSpace(item.NextAction), nextAt, strings.TrimSpace(item.Notes), now, item.ID)
		if err != nil {
			return web.Opportunity{}, err
		}
		count, err := result.RowsAffected()
		if err != nil {
			return web.Opportunity{}, err
		}
		if count == 0 {
			return web.Opportunity{}, sql.ErrNoRows
		}
		var created int64
		if err := repo.db.QueryRowContext(ctx, `SELECT created_at FROM crm_opportunities WHERE id = ?`, item.ID).Scan(&created); err != nil {
			return web.Opportunity{}, err
		}
		item.CreatedAt = time.Unix(created, 0).UTC()
	}
	item.NextAction = strings.TrimSpace(item.NextAction)
	item.Notes = strings.TrimSpace(item.Notes)
	item.UpdatedAt = updatedAt
	if err := repo.db.QueryRowContext(ctx, `SELECT name FROM crm_offers WHERE id = ?`, item.OfferID).Scan(&item.OfferName); err != nil {
		return web.Opportunity{}, fmt.Errorf("load opportunity offer: %w", err)
	}
	return item, nil
}

func (repo *repo) DeleteOpportunity(ctx context.Context, id int64) error {
	if id <= 0 {
		return errors.New("invalid opportunity id")
	}
	result, err := repo.db.ExecContext(ctx, `DELETE FROM crm_opportunities WHERE id = ?`, id)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return sql.ErrNoRows
	}
	return nil
}

type opportunityScanner interface{ Scan(...any) error }

func scanOpportunity(row opportunityScanner) (web.Opportunity, error) {
	var item web.Opportunity
	var stage string
	var price, monthlyPrice, nextAt sql.NullInt64
	var created, updated int64
	if err := row.Scan(&item.ID, &item.LeadKey, &item.OfferID, &item.OfferName, &price, &monthlyPrice, &stage, &item.NextAction, &nextAt, &item.Notes, &created, &updated); err != nil {
		return web.Opportunity{}, err
	}
	item.Stage = web.CRMStage(stage)
	if price.Valid {
		value := price.Int64
		item.PriceCents = &value
	}
	if monthlyPrice.Valid {
		value := monthlyPrice.Int64
		item.MonthlyPriceCents = &value
	}
	if nextAt.Valid {
		value := time.Unix(nextAt.Int64, 0).UTC()
		item.NextActionAt = &value
	}
	item.CreatedAt = time.Unix(created, 0).UTC()
	item.UpdatedAt = time.Unix(updated, 0).UTC()
	return item, nil
}
