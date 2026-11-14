// Package saml implements the SAML SP (metadata, login, ACS) for one IdP.
package saml

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/crewjam/saml/samlsp"
	dsig "github.com/russellhaering/goxmldsig"

	crewsaml "github.com/crewjam/saml"
)

// Service wraps crewjam's ServiceProvider.
type Service struct {
	sp      *crewsaml.ServiceProvider
	webBase string
}

// Config configures the SP.
type Config struct {
	BaseURL     string // e.g. http://localhost:18080
	EntityID    string // must match the IdP client id
	IDPMetadata string // URL reachable from the API
	KeyFile     string
	Certificate string
	WebURL      string
}

// New fetches IdP metadata and builds the SP. Missing key/cert are generated
// in memory (dev only; the SP certificate changes on restart).
func New(ctx context.Context, cfg Config) (*Service, error) {
	base, err := url.Parse(cfg.BaseURL)
	if err != nil || base.Host == "" {
		return nil, fmt.Errorf("saml: AIP_SAML_BASE_URL is required")
	}
	idpURL, err := url.Parse(cfg.IDPMetadata)
	if err != nil || idpURL.Host == "" {
		return nil, fmt.Errorf("saml: AIP_SAML_IDP_METADATA is required")
	}
	idp, err := samlsp.FetchMetadata(ctx, http.DefaultClient, *idpURL)
	if err != nil {
		return nil, fmt.Errorf("saml: fetch IdP metadata: %w", err)
	}
	key, cert, err := loadOrGenerate(cfg.KeyFile, cfg.Certificate)
	if err != nil {
		return nil, err
	}
	m, err := samlsp.New(samlsp.Options{
		EntityID: cfg.EntityID, URL: *base, Key: key, Certificate: cert,
		IDPMetadata: idp, AllowIDPInitiated: false, SignRequest: true,
	})
	if err != nil {
		return nil, err
	}
	// Keycloak rejects SHA-1 signatures; use SHA-256 for Authn/Logout requests.
	m.ServiceProvider.SignatureMethod = dsig.RSASHA256SignatureMethod
	return &Service{sp: &m.ServiceProvider, webBase: cfg.WebURL}, nil
}

func loadOrGenerate(keyFile, certFile string) (*rsa.PrivateKey, *x509.Certificate, error) {
	if keyFile != "" && certFile != "" {
		keyPEM, err := os.ReadFile(keyFile) //nolint:gosec // operator-provided path
		if err != nil {
			return nil, nil, err
		}
		block, _ := pem.Decode(keyPEM)
		if block == nil {
			return nil, nil, fmt.Errorf("saml: key is not PEM")
		}
		keyAny, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			if k, rerr := x509.ParsePKCS1PrivateKey(block.Bytes); rerr == nil {
				keyAny = k
			} else {
				return nil, nil, err
			}
		}
		key, ok := keyAny.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("saml: key is not RSA")
		}
		certPEM, err := os.ReadFile(certFile) //nolint:gosec // operator-provided path
		if err != nil {
			return nil, nil, err
		}
		cblock, _ := pem.Decode(certPEM)
		if cblock == nil {
			return nil, nil, fmt.Errorf("saml: certificate is not PEM")
		}
		cert, err := x509.ParseCertificate(cblock.Bytes)
		if err != nil {
			return nil, nil, err
		}
		return key, cert, nil
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "aip-saml-sp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, nil, err
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		return nil, nil, err
	}
	return key, cert, nil
}

// Metadata returns the SP EntityDescriptor XML.
func (s *Service) Metadata() ([]byte, error) {
	return xml.MarshalIndent(s.sp.Metadata(), "", "  ")
}

// LoginURL returns the IdP redirect URL and the request id to validate.
func (s *Service) LoginURL() (string, string, error) {
	req, err := s.sp.MakeAuthenticationRequest(s.sp.GetSSOBindingLocation(crewsaml.HTTPRedirectBinding), crewsaml.HTTPRedirectBinding, crewsaml.HTTPPostBinding)
	if err != nil {
		return "", "", err
	}
	// RelayState carries the request id; the assertion is still signature-checked.
	target, err := req.Redirect(req.ID, s.sp)
	if err != nil {
		return "", "", err
	}
	return target.String(), req.ID, nil
}

// Parse validates an ACS response against the stored request id.
func (s *Service) Parse(r *http.Request, requestID string) (*crewsaml.Assertion, error) {
	return s.sp.ParseResponse(r, []string{requestID})
}

// Identity extracts the NameID/email from an assertion.
func Identity(assertion *crewsaml.Assertion) (subject, email, name string, groups []string, err error) {
	if assertion.Subject == nil || assertion.Subject.NameID == nil {
		return "", "", "", nil, fmt.Errorf("saml: assertion has no NameID")
	}
	subject = assertion.Subject.NameID.Value
	email = subject
	name = ""
	for _, statement := range assertion.AttributeStatements {
		for _, attr := range statement.Attributes {
			value := ""
			if len(attr.Values) > 0 {
				value = attr.Values[0].Value
			}
			switch attr.Name {
			case "groups", "Group", "Role", "memberOf", "http://schemas.xmlsoap.org/claims/Group":
				if value != "" {
					groups = append(groups, value)
				}
			case "email", "urn:oid:0.9.2342.19200300.100.1.3", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress":
				if value != "" {
					email = value
				}
			case "displayName", "urn:oid:2.16.840.1.113730.3.1.241", "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name":
				name = value
			}
		}
	}
	return subject, email, name, groups, nil
}

// WebURL is the post-login redirect base.
func (s *Service) WebURL() string { return s.webBase }

// PostLogout returns an auto-submitting HTML form for the IdP SLO POST endpoint.
func (s *Service) PostLogout(nameID string) ([]byte, error) {
	if s.sp.GetSLOBindingLocation(crewsaml.HTTPPostBinding) == "" {
		return nil, fmt.Errorf("saml: IdP has no HTTP-POST SLO endpoint")
	}
	return s.sp.MakePostLogoutRequest(nameID, "")
}
