// Package migrations embeds the SQL migrations into the binary so that
// `aipctl migrate` and the deployment image need no external files.
package migrations

import "embed"

// FS contains every migration in this directory.
//
//go:embed *.sql
var FS embed.FS
