package notifications

import (
	"context"
	"testing"
)

type fakeStore struct {
	inserted []string
	admins   []Admin
	readIDs  []string
}

func (f *fakeStore) Insert(_ context.Context, _, userID, kind, title, body, link string) (Notification, error) {
	f.inserted = append(f.inserted, userID+":"+title)
	return Notification{ID: "n1", Type: kind, Title: title, Body: body, Link: link}, nil
}
func (f *fakeStore) List(context.Context, string, string, bool, int) ([]Notification, error) {
	return nil, nil
}
func (f *fakeStore) MarkRead(_ context.Context, _, _ string, ids []string) error {
	f.readIDs = ids
	return nil
}
func (f *fakeStore) Admins(context.Context, string) ([]Admin, error) { return f.admins, nil }

type fakeEmailer struct{ sent []string }

func (f *fakeEmailer) SendNotification(_ context.Context, to, title, _, link string) error {
	f.sent = append(f.sent, to+"|"+title+"|"+link)
	return nil
}

func TestNotifyAdminsStoresAndEmails(t *testing.T) {
	store := &fakeStore{admins: []Admin{{UserID: "u1", Email: "a@example.com"}, {UserID: "u2"}}}
	emailer := &fakeEmailer{}
	svc := New(store, nil).WithEmailer(emailer, "https://app.test")
	if err := svc.NotifyAdmins(context.Background(), "o", "approval", "Plan ready", "body", "/projects/p/runs/r"); err != nil {
		t.Fatal(err)
	}
	if len(store.inserted) != 2 {
		t.Fatalf("inserted = %v", store.inserted)
	}
	if len(emailer.sent) != 1 || emailer.sent[0] != "a@example.com|Plan ready|https://app.test/projects/p/runs/r" {
		t.Fatalf("sent = %v", emailer.sent)
	}
}
