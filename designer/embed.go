// Package designer embeds the report designer, its ready-made templates and the
// template JSON Schema so `ddreport -serve` can host them.
package designer

import "embed"

//go:embed designer.js index.html bootstrap.html templates template.schema.json
var Files embed.FS
