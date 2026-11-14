package usage

import (
	"context"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/agent"
	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	price    *Price
	recorded Entry
	upserted Price
}

func (f *fakeStore) Record(_ context.Context, e Entry) error { f.recorded = e; return nil }
func (f *fakeStore) Cost(context.Context, string, int) (float64, int, error) {
	return 0, 0, nil
}
func (f *fakeStore) Price(_ context.Context, _, _, _ string) (Price, bool, error) {
	if f.price != nil {
		return *f.price, true, nil
	}
	return Price{}, false, nil
}
func (f *fakeStore) UpsertPrice(_ context.Context, _ string, p Price) (Price, error) {
	f.upserted = p
	return p, nil
}
func (f *fakeStore) ListForRun(context.Context, string, string) ([]Entry, error) { return nil, nil }
func (f *fakeStore) ListPrices(context.Context, string) ([]Price, error)         { return nil, nil }

func TestRecordUsesTablePrice(t *testing.T) {
	store := &fakeStore{price: &Price{InputPer1K: 10, OutputPer1K: 20}}
	svc := New(store, 1, 2) // config fallback must lose to the table
	if err := svc.Record(context.Background(), domain.Run{ID: "r", OrgID: "o"},
		agent.Result{Provider: "p", Model: "m", InputTokens: 1000, OutputTokens: 500}); err != nil {
		t.Fatal(err)
	}
	if store.recorded.Cost != 20 { // 1000*10/1000 + 500*20/1000 = 10 + 10
		t.Fatalf("cost = %v, want 20", store.recorded.Cost)
	}
}

func TestRecordFallsBackToConfigPrice(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, 1, 2)
	if err := svc.Record(context.Background(), domain.Run{ID: "r", OrgID: "o"},
		agent.Result{Provider: "p", Model: "m", InputTokens: 1000, OutputTokens: 0}); err != nil {
		t.Fatal(err)
	}
	if store.recorded.Cost != 1 {
		t.Fatalf("cost = %v, want 1", store.recorded.Cost)
	}
}

func TestRecordSkipsZeroUsage(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, 1, 2)
	if err := svc.Record(context.Background(), domain.Run{OrgID: "o"}, agent.Result{}); err != nil {
		t.Fatal(err)
	}
	if store.recorded != (Entry{}) {
		t.Fatalf("recorded = %+v, want nothing", store.recorded)
	}
}
