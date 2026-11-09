// Package migrations holds the plain SQL migrations, applied in file order by
// internal/migrate (ADR 0003). Files are NNNN_name.sql and never edited once
// merged: a mistake is fixed by the next migration.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
