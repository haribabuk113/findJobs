// Package migrations embeds the SQL migrations so the binary applies them
// itself at startup (goose); dev_seed/ is excluded and stays a manual load.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
