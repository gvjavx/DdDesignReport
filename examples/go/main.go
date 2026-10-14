// Run from the repository root: go run ./examples/go
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	ddreport "github.com/gvjavx/DdDesignReport"
)

func main() {
	template, err := os.ReadFile("examples/sales.json")
	if err != nil {
		log.Fatal(err)
	}
	raw, err := os.ReadFile("examples/data.json")
	if err != nil {
		log.Fatal(err)
	}
	// data can be any value that marshals to JSON: a struct, a map, or raw JSON like here
	data := json.RawMessage(raw)

	os.MkdirAll("examples/out", 0o755)
	for _, format := range []string{"pdf", "html", "xlsx", "csv"} {
		out, err := ddreport.Render(template, data, format)
		if err != nil {
			log.Fatal(err)
		}
		name := "examples/out/go-sales." + format
		if err := os.WriteFile(name, out, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("wrote", name)
	}
}
