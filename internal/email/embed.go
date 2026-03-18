package email

import "embed"

//go:embed "templates/*.tmpl"
var FS embed.FS
