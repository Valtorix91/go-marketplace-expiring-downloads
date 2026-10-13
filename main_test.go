package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOrderHandoff(t *testing.T) {
	tests := []struct {
		name, body string
		status     int
		want       string
	}{
		{"valid order", `{"asset_key":"books/etl.pdf","buyer_email":"buyer@example.com"}`, 200, "download_url"},
		{"missing asset", `{"buyer_email":"buyer@example.com"}`, 400, "required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Write([]byte(`{"ok":true,"data":{"url":"https://signed.example/download"}}`))
			}))
			defer srv.Close()
			client := &infraiClient{base: srv.URL, key: "test", http: srv.Client()}
			req := httptest.NewRequest("POST", "/orders", strings.NewReader(tc.body))
			rec := httptest.NewRecorder()
			handler(client).ServeHTTP(rec, req)
			if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.want) {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
