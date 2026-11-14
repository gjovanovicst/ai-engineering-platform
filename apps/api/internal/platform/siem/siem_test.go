package siem

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendSignsPayload(t *testing.T) {
	var body []byte
	var signature string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = io.ReadAll(r.Body)
		signature = r.Header.Get("X-AIP-Signature")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()
	sender := &Sender{URL: srv.URL, Secret: []byte("key")}
	if err := sender.Send(context.Background(), []byte(`{"id":1}`)); err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"id":1}` || signature == "" {
		t.Fatalf("body=%q signature=%q", body, signature)
	}
}
