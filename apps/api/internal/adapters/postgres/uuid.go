package postgres

import (
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func uuidPtr(u pgtype.UUID) *string {
	if !u.Valid {
		return nil
	}
	value := uuidString(u)
	return &value
}

func pgTime(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
