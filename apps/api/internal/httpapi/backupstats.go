package httpapi

import "context"

// BackupStats exposes backup heartbeat age.
type BackupStats interface {
	LastBaseBackupAge(ctx context.Context) (float64, error)
	LastRestoreAge(ctx context.Context) (float64, error)
	LastOffsiteAge(ctx context.Context) (float64, error)
}
