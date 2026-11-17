package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProspectAPIRejectsUnknownStage(t *testing.T) {
	srv, err := New(NewService(nil, t.TempDir()), ":0")
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/prospects/"+key, strings.NewReader(`{"stage":"run-this"}`))
	rec := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid stage to return %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}
}

func TestProspectAPIRejectsTrailingJSON(t *testing.T) {
	srv, err := New(NewService(nil, t.TempDir()), ":0")
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	body := `{"stage":"new"} {"stage":"customer"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/prospects/"+key, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected trailing JSON to return %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestProspectAPIRejectsUnknownFields(t *testing.T) {
	srv, err := New(NewService(nil, t.TempDir()), ":0")
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	body := `{"stage":"new","unexpected":true}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/prospects/"+key, strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected unknown JSON fields to return %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

func TestProspectAPIRejectsOversizedTouchBody(t *testing.T) {
	srv, err := New(NewService(nil, t.TempDir()), ":0")
	if err != nil {
		t.Fatal(err)
	}
	key := strings.Repeat("a", 64)
	body := `{"stage":"new","message":"` + strings.Repeat("x", 5000) + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/prospects/"+key+"/touches", strings.NewReader(body))
	rec := httptest.NewRecorder()
	srv.srv.Handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected oversized body to return %d, got %d", http.StatusBadRequest, rec.Code)
	}
}
