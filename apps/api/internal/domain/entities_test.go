package domain

import (
	"errors"
	"testing"
)

func TestNewOrg(t *testing.T) {
	tests := []struct {
		name    string
		orgName string
		slug    string
		wantErr error
	}{
		{"valid", "Acme", "acme", nil},
		{"trims and lowercases", "  Acme  ", "ACME", nil},
		{"empty name", "  ", "acme", ErrValidation},
		{"slug too short", "Acme", "ab", ErrValidation},
		{"slug with underscore", "Acme", "acme_inc", ErrValidation},
		{"slug with uppercase is normalized", "Acme", "AcmeInc", nil},
		{"slug with spaces", "Acme", "acme inc", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewOrg(tc.orgName, tc.slug)
			assertErr(t, err, tc.wantErr)
		})
	}
}

func TestOrgMaxConcurrentRuns(t *testing.T) {
	o, err := NewOrg("Acme", "acme")
	if err != nil {
		t.Fatal(err)
	}
	o.MaxConcurrentRuns = 0
	assertErr(t, o.Validate(), ErrValidation)
	o.MaxConcurrentRuns = 65
	assertErr(t, o.Validate(), ErrValidation)
	o.MaxConcurrentRuns = 8
	if err := o.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
}

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"  Goran@Acme.IO ", "goran@acme.io", false},
		{"goran@acme.io", "goran@acme.io", false},
		{"", "", true},
		{"not-an-email", "", true},
		{"two words@acme.io", "", true},
	}
	for _, tc := range tests {
		got, err := NormalizeEmail(tc.in)
		if tc.wantErr {
			assertErr(t, err, ErrValidation)
			continue
		}
		if err != nil {
			t.Fatalf("NormalizeEmail(%q) error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNewUser(t *testing.T) {
	u, err := NewUser("Goran@Acme.IO", " Goran ")
	if err != nil {
		t.Fatalf("NewUser() error: %v", err)
	}
	if u.Email != "goran@acme.io" {
		t.Errorf("Email = %q, want normalized", u.Email)
	}
	if u.Name != "Goran" {
		t.Errorf("Name = %q, want trimmed", u.Name)
	}
	_, err = NewUser("bad", "Goran")
	assertErr(t, err, ErrValidation)
}

func TestNewProject(t *testing.T) {
	tests := []struct {
		name    string
		orgID   string
		pName   string
		slug    string
		wantErr error
	}{
		{"valid", "org-1", "CRM Platform", "crm-platform", nil},
		{"missing org", "", "CRM", "crm", ErrValidation},
		{"missing name", "org-1", " ", "crm", ErrValidation},
		{"bad slug", "org-1", "CRM", "CRM Platform", ErrValidation},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewProject(tc.orgID, tc.pName, tc.slug)
			assertErr(t, err, tc.wantErr)
		})
	}
}

func assertErr(t *testing.T, err, want error) {
	t.Helper()
	if want == nil {
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("expected error %v, got nil", want)
	}
	if !errors.Is(err, want) {
		t.Fatalf("error = %v, want errors.Is(_, %v)", err, want)
	}
}
