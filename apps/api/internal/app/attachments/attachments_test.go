package attachments

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type fakeStore struct {
	in  CreateInput
	sha string
}

func (f *fakeStore) Create(_ context.Context, _, _, _, _ string, in CreateInput, sha string) (Meta, error) {
	f.in, f.sha = in, sha
	return Meta{Filename: in.Filename, ContentType: in.ContentType, SizeBytes: len(in.Data), SHA256: sha}, nil
}
func (f *fakeStore) List(context.Context, string, string) ([]Meta, error) { return nil, nil }
func (f *fakeStore) Get(context.Context, string, string, string) (Attachment, error) {
	return Attachment{}, nil
}

func TestUploadValidatesAndHashes(t *testing.T) {
	store := &fakeStore{}
	svc := New(store)
	meta, err := svc.Upload(context.Background(), "o", "p", "t", "u", "../shot.png", "image/png", []byte("png"))
	if err != nil {
		t.Fatal(err)
	}
	if meta.Filename != "shot.png" {
		t.Fatalf("filename = %q", meta.Filename)
	}
	if len(store.sha) != 64 {
		t.Fatalf("sha = %q", store.sha)
	}
}

func TestUploadRejectsTypeAndSize(t *testing.T) {
	svc := New(&fakeStore{})
	if _, err := svc.Upload(context.Background(), "o", "p", "t", "u", "x.exe", "application/x-msdownload", []byte("x")); err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported type, got %v", err)
	}
	big := make([]byte, MaxBytes+1)
	if _, err := svc.Upload(context.Background(), "o", "p", "t", "u", "big.png", "image/png", big); err == nil {
		t.Fatal("expected size error")
	}
}

type rejectScanner struct{}

func (rejectScanner) Scan(context.Context, []byte) error { return errors.New("EICAR found") }

func TestUploadRejectsMalware(t *testing.T) {
	svc := New(&fakeStore{}).WithScanner(rejectScanner{})
	_, err := svc.Upload(context.Background(), "o", "p", "t", "u", "x.txt", "text/plain", []byte("bad"))
	if err == nil || !strings.Contains(err.Error(), "malware") {
		t.Fatalf("expected malware error, got %v", err)
	}
}

type memoryObjects struct{ data map[string][]byte }

func (m *memoryObjects) Put(_ context.Context, key string, body io.Reader, _ int64, _ string) error {
	data, err := io.ReadAll(body)
	if err != nil {
		return err
	}
	m.data[key] = data
	return nil
}
func (m *memoryObjects) Get(_ context.Context, key string) ([]byte, error) { return m.data[key], nil }

func TestUploadUsesObjectStore(t *testing.T) {
	store := &fakeStore{}
	objects := &memoryObjects{data: map[string][]byte{}}
	svc := New(store).WithObjectStore(objects)
	if _, err := svc.Upload(context.Background(), "o", "p", "t", "u", "x.txt", "text/plain", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	if store.in.ObjectKey == "" || store.in.Data != nil {
		t.Fatalf("input = %+v", store.in)
	}
}
