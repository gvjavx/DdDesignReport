package ddreport

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

// Large reports, to see what a big report costs and keep it from getting slower:
//
//	go test -run '^$' -bench Large -benchmem
//
// The 100 000 row case only runs with DDREPORT_BENCH_BIG=1, as it takes a while.
func BenchmarkLarge(b *testing.B) {
	template, _ := os.ReadFile("designer/templates/sales-report.json")
	sizes := []int{10_000}
	if os.Getenv("DDREPORT_BENCH_BIG") != "" {
		sizes = append(sizes, 100_000)
	}
	for _, n := range sizes {
		tpl, data := largeReport(b, template, n)
		for _, format := range []string{"pdf", "html", "xlsx", "xls", "csv"} {
			b.Run(fmt.Sprintf("%s/%d", format, n), func(b *testing.B) {
				for b.Loop() {
					out, err := Render(tpl, data, format)
					if err != nil {
						b.Fatal(err)
					}
					b.ReportMetric(float64(len(out))/1e6, "MB-out")
				}
			})
		}
	}
}

// largeReport is the ready-made sales report with n rows, spread over a dozen categories.
func largeReport(tb testing.TB, file []byte, n int) ([]byte, map[string]any) {
	var f struct {
		Template map[string]any
		Data     map[string]any
	}
	if err := json.Unmarshal(file, &f); err != nil {
		tb.Fatal(err)
	}
	rows := make([]any, n)
	for i := range rows {
		rows[i] = map[string]any{
			"category": fmt.Sprintf("Category %02d", i%12),
			"name":     fmt.Sprintf("Product %d", i),
			"qty":      float64(i%7 + 1),
			"price":    float64(1000 + i%5000),
		}
	}
	f.Data["rows"] = rows
	tpl, _ := json.Marshal(f.Template)
	return tpl, f.Data
}
