package web

import (
	"strings"
	"testing"
)

func TestPlaceKeyIsStableAndDoesNotExposeGoogleURL(t *testing.T) {
	place := Place{Title: "A1 Vistoria", Link: "https://maps.google.com/?cid=private-example"}
	first := placeKey(place)
	second := placeKey(place)

	if first != second {
		t.Fatalf("placeKey should be stable, got %q and %q", first, second)
	}
	if len(first) != 64 || strings.Contains(first, "maps.google") {
		t.Fatalf("placeKey should be a SHA-256 hex digest, got %q", first)
	}
	if first == placeKey(Place{Title: "Different place", Link: "https://maps.google.com/?cid=other"}) {
		t.Fatal("different places should have different keys")
	}
}

func TestParsePlacesAddsStableProspectKey(t *testing.T) {
	csvData := `title,latitude,longitude,link,address,category,complete_address
Example Cafe,-1.45,-48.49,https://maps.google.com/?cid=example,Rua Um,Cafe,{}`
	places, err := parsePlaces(strings.NewReader(csvData))
	if err != nil {
		t.Fatal(err)
	}
	if len(places) != 1 {
		t.Fatalf("expected one place, got %d", len(places))
	}
	if places[0].Key == "" || places[0].Key != placeKey(places[0]) {
		t.Fatalf("expected parsed place to have a stable key, got %q", places[0].Key)
	}
}

func TestProspectStageValidation(t *testing.T) {
	for _, stage := range []ProspectStage{
		StageNew,
		StageFirstContact,
		StageFollowUpOne,
		StageFollowUpTwo,
		StageReplied,
		StageCustomer,
		StageNotInterested,
	} {
		if !stage.Valid() {
			t.Errorf("expected stage %q to be valid", stage)
		}
	}

	if ProspectStage("run-this").Valid() {
		t.Fatal("unknown stage must be rejected")
	}
}
