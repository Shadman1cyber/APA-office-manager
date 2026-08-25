package backend

import "embed"

//go:embed all:migrations
var MigrationsFS embed.FS
