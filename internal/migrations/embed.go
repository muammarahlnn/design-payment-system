// Package migrations embeds the SQL migration files so they ship inside the
// compiled binary — no extra files need to be mounted or copied at runtime.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
