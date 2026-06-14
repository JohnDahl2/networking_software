package migrations

import "embed"

// Files contains the embedded SQL migration files.
//
//go:embed *.sql
var Files embed.FS
