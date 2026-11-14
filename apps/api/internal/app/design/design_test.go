package design

import (
	"context"
	"errors"
	"testing"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

type fakeStore struct {
	tokens     []Token
	components []Component
}

func (f *fakeStore) ListTokens(context.Context, string) ([]Token, error) { return f.tokens, nil }
func (f *fakeStore) CreateToken(_ context.Context, _ string, t Token) (Token, error) {
	f.tokens = append(f.tokens, t)
	return t, nil
}
func (f *fakeStore) UpdateToken(_ context.Context, _, _ string, t Token) (Token, error) {
	return t, nil
}
func (f *fakeStore) DeleteToken(context.Context, string, string) error { return nil }
func (f *fakeStore) CountTokens(context.Context, string) (int, error)  { return len(f.tokens), nil }
func (f *fakeStore) ListComponents(context.Context, string) ([]Component, error) {
	return f.components, nil
}
func (f *fakeStore) CreateComponent(_ context.Context, _ string, c Component) (Component, error) {
	f.components = append(f.components, c)
	return c, nil
}
func (f *fakeStore) UpdateComponent(_ context.Context, _, _ string, c Component) (Component, error) {
	return c, nil
}
func (f *fakeStore) DeleteComponent(context.Context, string, string) error { return nil }
func (f *fakeStore) CountComponents(context.Context, string) (int, error) {
	return len(f.components), nil
}

func TestCreateTokenValidates(t *testing.T) {
	svc := New(&fakeStore{}, nil)
	if _, err := svc.CreateToken(context.Background(), "o", Token{Name: "", Value: "x"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty name err = %v", err)
	}
	if _, err := svc.CreateToken(context.Background(), "o", Token{Name: "x", Value: ""}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty value err = %v", err)
	}
	if _, err := svc.CreateToken(context.Background(), "o", Token{Name: "x", Value: "1", Category: "nope"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad category err = %v", err)
	}
	if _, err := svc.CreateToken(context.Background(), "o", Token{Name: "x", Value: "#fff"}); err != nil {
		t.Fatalf("valid token err = %v", err)
	}
}

func TestSeedDefaultsIsIdempotent(t *testing.T) {
	store := &fakeStore{}
	svc := New(store, nil)
	tokens, components, err := svc.SeedDefaults(context.Background(), "o")
	if err != nil {
		t.Fatal(err)
	}
	if tokens == 0 || components == 0 {
		t.Fatalf("seeded tokens=%d components=%d", tokens, components)
	}
	secondTokens, secondComponents, err := svc.SeedDefaults(context.Background(), "o")
	if err != nil {
		t.Fatal(err)
	}
	if secondTokens != 0 || secondComponents != 0 {
		t.Fatalf("second seed created tokens=%d components=%d", secondTokens, secondComponents)
	}
}

func TestCreateComponentValidatesStatus(t *testing.T) {
	svc := New(&fakeStore{}, nil)
	if _, err := svc.CreateComponent(context.Background(), "o", Component{Name: ""}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("empty name err = %v", err)
	}
	if _, err := svc.CreateComponent(context.Background(), "o", Component{Name: "x", Status: "weird"}); !errors.Is(err, domain.ErrValidation) {
		t.Fatalf("bad status err = %v", err)
	}
}
