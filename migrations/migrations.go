// Package migrations embeds the numbered SQL migration files
// (NNNN_name.sql) applied in order by internal/db.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
