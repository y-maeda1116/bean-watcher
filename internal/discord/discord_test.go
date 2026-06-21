package discord

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendSuccess(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type: got %s", ct)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.URL, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if !strings.Contains(gotBody, `"content":"hello"`) {
		t.Errorf("body: got %s", gotBody)
	}
}

func TestSendErrorOnNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	if err := Send(context.Background(), srv.URL, "x"); err == nil {
		t.Error("expected error on 400")
	}
}

func TestSendMasksURLInError(t *testing.T) {
	secret := "https://discord.com/api/webhooks/SECRET_TOKEN_123"
	// 不正なホストで接続エラーを発生させる
	err := Send(context.Background(), "http://127.0.0.1:0/webhook", "x")
	if err == nil {
		t.Skip("connection did not fail")
	}
	if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks URL: %v", err)
	}
}

func TestSendEmptyURL(t *testing.T) {
	if err := Send(context.Background(), "", "x"); err == nil {
		t.Error("expected error for empty URL")
	}
}
