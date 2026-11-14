// Package samlmeta parses SAML IdP metadata for configuration.
package samlmeta

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Metadata is the subset we store.
type Metadata struct {
	EntityID    string
	Certificate string // PEM
	SSOURL      string
}

const redirectBinding = "urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"

type entityDescriptor struct {
	EntityID         string           `xml:"entityID,attr"`
	IDPSSODescriptor idpSSODescriptor `xml:"IDPSSODescriptor"`
}

type idpSSODescriptor struct {
	KeyDescriptors      []keyDescriptor `xml:"KeyDescriptor"`
	SingleSignOnService []endpoint      `xml:"SingleSignOnService"`
}

type endpoint struct {
	Binding  string `xml:"Binding,attr"`
	Location string `xml:"Location,attr"`
}

type keyDescriptor struct {
	Use     string  `xml:"use,attr"`
	KeyInfo keyInfo `xml:"KeyInfo"`
}

type keyInfo struct {
	X509Data x509Data `xml:"X509Data"`
}

type x509Data struct {
	Certificates []string `xml:"X509Certificate"`
}

// Parse extracts the entity id and first signing certificate.
func Parse(raw []byte) (Metadata, error) {
	var doc entityDescriptor
	if err := xml.Unmarshal(raw, &doc); err != nil {
		return Metadata{}, fmt.Errorf("saml metadata: %w", err)
	}
	if strings.TrimSpace(doc.EntityID) == "" {
		return Metadata{}, fmt.Errorf("saml metadata: entityID is missing")
	}
	for _, key := range doc.IDPSSODescriptor.KeyDescriptors {
		if key.Use != "" && key.Use != "signing" {
			continue
		}
		if len(key.KeyInfo.X509Data.Certificates) == 0 {
			continue
		}
		der, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(key.KeyInfo.X509Data.Certificates[0]), ""))
		if err != nil {
			return Metadata{}, fmt.Errorf("saml metadata: certificate base64: %w", err)
		}
		if _, err := x509.ParseCertificate(der); err != nil {
			return Metadata{}, fmt.Errorf("saml metadata: certificate: %w", err)
		}
		ssoURL := ""
		for _, endpoint := range doc.IDPSSODescriptor.SingleSignOnService {
			if endpoint.Binding == redirectBinding && endpoint.Location != "" {
				ssoURL = endpoint.Location
				break
			}
		}
		return Metadata{
			EntityID:    doc.EntityID,
			Certificate: string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})),
			SSOURL:      ssoURL,
		}, nil
	}
	return Metadata{}, fmt.Errorf("saml metadata: no signing certificate")
}

// CheckURL rejects non-http(s) and (unless allowed) private/loopback targets.
// This is an explicit SSRF guard for admin-supplied metadata URLs.
func CheckURL(rawURL string, allowPrivate bool) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return fmt.Errorf("saml metadata: metadata_url must be an http(s) URL")
	}
	host := parsed.Hostname()
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("saml metadata: resolve %s: %w", host, err)
	}
	if allowPrivate {
		return nil
	}
	for _, ip := range ips {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			return fmt.Errorf("saml metadata: %s resolves to a private address; set AIP_SSO_ALLOW_PRIVATE_METADATA=true for an internal IdP", host)
		}
	}
	return nil
}

// Fetch downloads and parses metadata (10s, 1 MiB limit).
func Fetch(ctx context.Context, rawURL string, allowPrivate bool) (Metadata, error) {
	if err := CheckURL(rawURL, allowPrivate); err != nil {
		return Metadata{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Metadata{}, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Metadata{}, fmt.Errorf("saml metadata: fetch: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return Metadata{}, fmt.Errorf("saml metadata: status %d", res.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return Metadata{}, err
	}
	return Parse(raw)
}
