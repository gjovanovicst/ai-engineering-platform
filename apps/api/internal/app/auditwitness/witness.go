// Package auditwitness writes an external anchor of the audit chain.
package auditwitness

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/gjovanovicst/ai-engineering-platform/internal/domain"
)

// Head is the current audit chain position.
type Head struct {
	Rows     int64
	LastID   int64
	LastHash string
}

// Witness is a stored anchor.
type Witness struct {
	ID        string
	ObjectKey string
	RowCount  int64
	LastID    int64
	LastHash  string
	CreatedAt time.Time
}

// Store persists witness metadata.
type Store interface {
	Head(ctx context.Context, orgID string) (Head, error)
	HashAt(ctx context.Context, orgID string, id int64) (string, error)
	Insert(ctx context.Context, orgID, key string, head Head) (Witness, error)
	Latest(ctx context.Context, orgID string) (Witness, error)
}

// Objects is the S3 subset.
type Objects interface {
	Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error
	Get(ctx context.Context, key string) ([]byte, error)
}

// Service writes and verifies witnesses.
type Service struct {
	store   Store
	objects Objects
}

// New wires the service.
func New(store Store, objects Objects) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("auditwitness: store is required")
	}
	return &Service{store: store, objects: objects}, nil
}

type payload struct {
	OrgID    string    `json:"org_id"`
	RowCount int64     `json:"row_count"`
	LastID   int64     `json:"last_id"`
	LastHash string    `json:"last_hash"`
	Written  time.Time `json:"written_at"`
}

// Create writes the current head to S3 and records the anchor.
func (s *Service) Create(ctx context.Context, orgID string) (Witness, error) {
	if s.objects == nil {
		return Witness{}, fmt.Errorf("%w: S3 is required for an external witness", domain.ErrValidation)
	}
	head, err := s.store.Head(ctx, orgID)
	if err != nil {
		return Witness{}, err
	}
	body, _ := json.Marshal(payload{OrgID: orgID, RowCount: head.Rows, LastID: head.LastID, LastHash: head.LastHash, Written: time.Now().UTC()})
	key := fmt.Sprintf("audit-witness/%s/%d.json", orgID, time.Now().UnixNano())
	if err := s.objects.Put(ctx, key, bytes.NewReader(body), int64(len(body)), "application/json"); err != nil {
		return Witness{}, err
	}
	return s.store.Insert(ctx, orgID, key, head)
}

// Verify compares the latest witness with the current chain.
func (s *Service) Verify(ctx context.Context, orgID string) (Witness, bool, error) {
	witness, err := s.store.Latest(ctx, orgID)
	if err != nil {
		return Witness{}, false, err
	}
	if s.objects == nil {
		return witness, false, fmt.Errorf("%w: S3 is required", domain.ErrValidation)
	}
	raw, err := s.objects.Get(ctx, witness.ObjectKey)
	if err != nil {
		return witness, false, err
	}
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return witness, false, err
	}
	head, err := s.store.Head(ctx, orgID)
	if err != nil {
		return witness, false, err
	}
	if head.Rows < p.RowCount || head.LastID < p.LastID {
		return witness, false, nil
	}
	if p.LastID == head.LastID {
		return witness, p.LastHash == head.LastHash, nil
	}
	at, err := s.store.HashAt(ctx, orgID, p.LastID)
	if err != nil {
		return witness, false, err
	}
	return witness, at == p.LastHash, nil
}
