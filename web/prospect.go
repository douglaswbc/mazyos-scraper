package web

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type ProspectStage string

const (
	StageNew            ProspectStage = "new"
	StageFirstContact   ProspectStage = "first_contact"
	StageFollowUpOne    ProspectStage = "follow_up_1"
	StageFollowUpTwo    ProspectStage = "follow_up_2"
	StageReplied        ProspectStage = "replied"
	StageCustomer       ProspectStage = "customer"
	StageNotInterested  ProspectStage = "not_interested"
	ProspectTouchOpened               = "opened"
)

func (s ProspectStage) Valid() bool {
	switch s {
	case StageNew, StageFirstContact, StageFollowUpOne, StageFollowUpTwo, StageReplied, StageCustomer, StageNotInterested:
		return true
	default:
		return false
	}
}

func ValidProspectKey(key string) bool {
	if len(key) != 64 {
		return false
	}
	_, err := hex.DecodeString(key)
	return err == nil
}

type ProspectState struct {
	Key        string         `json:"key"`
	Stage      ProspectStage  `json:"stage"`
	UpdatedAt  time.Time      `json:"updated_at"`
	TouchCount int            `json:"touch_count"`
	LastTouch  *ProspectTouch `json:"last_touch,omitempty"`
}

type ProspectTouch struct {
	ID        int64         `json:"id"`
	Key       string        `json:"key"`
	Stage     ProspectStage `json:"stage"`
	Message   string        `json:"message"`
	Outcome   string        `json:"outcome"`
	CreatedAt time.Time     `json:"created_at"`
}

type ProspectRepository interface {
	ListProspects(context.Context, []string) (map[string]ProspectState, error)
	SetProspectStage(context.Context, string, ProspectStage) (ProspectState, error)
	AddProspectTouch(context.Context, string, ProspectStage, string) (ProspectTouch, error)
	ListProspectTouches(context.Context, string) ([]ProspectTouch, error)
}

// placeKey gives a result a stable opaque ID without exposing its Maps URL to UI state APIs.
func placeKey(place Place) string {
	identity := strings.TrimSpace(place.Link)
	if identity == "" {
		identity = strings.Join([]string{
			strings.TrimSpace(place.Title),
			strings.TrimSpace(place.Address),
			strings.TrimSpace(place.City),
			strings.TrimSpace(place.Category),
		}, "|")
	}
	if strings.Trim(identity, "|") == "" {
		return ""
	}

	sum := sha256.Sum256([]byte(identity))
	return hex.EncodeToString(sum[:])
}
