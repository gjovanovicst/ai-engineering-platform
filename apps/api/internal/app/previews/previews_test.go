package previews

import (
	"context"
	"testing"
	"time"
)

type fakeStore struct{ env Environment }

func (f *fakeStore) Create(_ context.Context, env Environment) (Environment, error) {
	env.ID = "p1"
	f.env = env
	return env, nil
}
func (f *fakeStore) List(context.Context, string, string) ([]Environment, error) {
	return []Environment{f.env}, nil
}
func (f *fakeStore) Get(context.Context, string, string) (Environment, error) { return f.env, nil }
func (f *fakeStore) SetStatus(_ context.Context, _, _, status, url, cid, detail string) error {
	f.env.Status, f.env.URL, f.env.ContainerID, f.env.Error = status, url, cid, detail
	return nil
}

type fakeDriver struct{ started, stopped bool }

func (f *fakeDriver) Start(context.Context, string, int) (string, string, error) {
	f.started = true
	return "http://127.0.0.1:1234", "cid", nil
}
func (f *fakeDriver) Stop(context.Context, string) error { f.stopped = true; return nil }

func TestCreateStartDestroy(t *testing.T) {
	store := &fakeStore{}
	driver := &fakeDriver{}
	svc, _ := New(store, driver)
	env, err := svc.Create(context.Background(), "o", "p", "pr-1", "nginx:alpine", 80, 10*time.Minute)
	if err != nil || env.Status != "requested" {
		t.Fatalf("env=%+v err=%v", env, err)
	}
	if err := svc.Start(context.Background(), "o", "p1"); err != nil {
		t.Fatal(err)
	}
	if !driver.started || store.env.Status != "running" || store.env.URL == "" {
		t.Fatalf("env=%+v started=%v", store.env, driver.started)
	}
	if err := svc.Destroy(context.Background(), "o", "p1"); err != nil {
		t.Fatal(err)
	}
	if !driver.stopped || store.env.Status != "destroyed" {
		t.Fatalf("env=%+v stopped=%v", store.env, driver.stopped)
	}
}
