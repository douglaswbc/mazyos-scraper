package sqlite

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gosom/google-maps-scraper/web"
)

func TestProspectStageAndTouchesPersist(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "prospects.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	store, ok := repository.(web.ProspectRepository)
	if !ok {
		t.Fatal("SQLite repository must implement ProspectRepository")
	}

	ctx := context.Background()
	key := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

	state, err := store.SetProspectStage(ctx, key, web.StageFirstContact)
	if err != nil {
		t.Fatal(err)
	}
	if state.Stage != web.StageFirstContact || state.Key != key {
		t.Fatalf("unexpected saved state: %+v", state)
	}

	message := "Oi, tudo bem? Posso te mandar uma ideia rápida?"
	touch, err := store.AddProspectTouch(ctx, key, web.StageFirstContact, message)
	if err != nil {
		t.Fatal(err)
	}
	if touch.Outcome != web.ProspectTouchOpened || touch.Message != message {
		t.Fatalf("unexpected touch: %+v", touch)
	}

	state, err = store.SetProspectStage(ctx, key, web.StageFollowUpOne)
	if err != nil {
		t.Fatal(err)
	}
	if state.Stage != web.StageFollowUpOne || state.TouchCount != 1 {
		t.Fatalf("stage change should retain the touch count, got %+v", state)
	}

	states, err := store.ListProspects(ctx, []string{key})
	if err != nil {
		t.Fatal(err)
	}
	if got := states[key]; got.Stage != web.StageFollowUpOne || got.TouchCount != 1 {
		t.Fatalf("unexpected prospect state after stage change: %+v", got)
	}

	touches, err := store.ListProspectTouches(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	if len(touches) != 1 || touches[0].Message != message {
		t.Fatalf("unexpected touch history: %+v", touches)
	}
}

func TestRepositoryRejectsMalformedProspectKeys(t *testing.T) {
	repository, err := New(filepath.Join(t.TempDir(), "prospects.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	store := repository.(web.ProspectRepository)
	invalidKey := strings.Repeat("z", 64)

	if _, err := store.SetProspectStage(context.Background(), invalidKey, web.StageNew); err == nil {
		t.Fatal("expected non-hex prospect key to be rejected")
	}
	if _, err := store.AddProspectTouch(context.Background(), invalidKey, web.StageNew, "hello"); err == nil {
		t.Fatal("expected non-hex prospect touch key to be rejected")
	}
}
