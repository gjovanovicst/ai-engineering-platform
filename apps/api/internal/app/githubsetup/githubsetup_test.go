package githubsetup

import (
	"context"
	"encoding/json"
	"net/url"
	"testing"
)

type fakeStore struct {
	app     Record
	states  map[string]string
	created int
}

func (f *fakeStore) Upsert(_ context.Context, _ string, in Record) error { f.app = in; return nil }
func (f *fakeStore) Get(context.Context, string) (Record, error)         { return f.app, nil }
func (f *fakeStore) SetInstallation(_ context.Context, _ string, id int64) error {
	f.app.InstallationID = id
	return nil
}
func (f *fakeStore) CreateState(_ context.Context, state, org string) error {
	f.states[state] = org
	f.created++
	return nil
}
func (f *fakeStore) ConsumeState(_ context.Context, state string) (string, error) {
	org, ok := f.states[state]
	if !ok {
		return "", assertErr{}
	}
	delete(f.states, state)
	return org, nil
}

type assertErr struct{}

func (assertErr) Error() string { return "not found" }

type fakeCipher struct{}

func (fakeCipher) Encrypt(s string) ([]byte, error) { return []byte(s), nil }
func (fakeCipher) Decrypt(b []byte) (string, error) { return string(b), nil }

func TestSetupConsumesStateAndInstalls(t *testing.T) {
	store := &fakeStore{states: map[string]string{}}
	svc := New(store, fakeCipher{}, "http://api", "http://web", "")
	result, err := svc.CreateManifest(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatal(err)
	}
	if store.created != 2 {
		t.Fatalf("states = %d", store.created)
	}
	var manifest map[string]any
	if err := json.Unmarshal(result.Manifest, &manifest); err != nil {
		t.Fatal(err)
	}
	rawURL, _ := manifest["setup_url"].(string)
	parsed, err := url.Parse(rawURL)
	if err != nil || rawURL == "" {
		t.Fatalf("setup_url = %q", rawURL)
	}
	status, err := svc.Setup(context.Background(), parsed.Query().Get("state"), 42)
	if err != nil || status.InstallationID != 42 || !status.Installed {
		t.Fatalf("status=%+v err=%v", status, err)
	}
}
