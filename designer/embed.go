// Package designer embeds the report designer and its ready-made templates so
// `ddreport -serve` can host them.
package designer

import "embed"

//go:embed designer.js index.html templates
var Files embed.FS
