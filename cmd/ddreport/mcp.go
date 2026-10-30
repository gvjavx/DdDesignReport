package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	ddreport "github.com/gvjavx/DdDesignReport"
	"github.com/gvjavx/DdDesignReport/designer"
)

// serveMCP is a Model Context Protocol server over stdio (one JSON-RPC message
// per line), so an AI assistant can read how templates are written, check the
// templates it writes, and render them. Add it to an assistant as the command
// `ddreport -mcp`.
// ponytail: tools only, no resources or prompts; add them when an assistant needs them
func serveMCP(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 64<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(sc.Bytes(), &msg) != nil || msg.ID == nil {
			continue // a notification, or not a request: nothing to answer
		}
		reply := map[string]any{"jsonrpc": "2.0", "id": msg.ID}
		if result, err := mcpCall(msg.Method, msg.Params); err != nil {
			reply["error"] = map[string]any{"code": -32601, "message": err.Error()}
		} else {
			reply["result"] = result
		}
		if err := enc.Encode(reply); err != nil {
			return err
		}
	}
	return sc.Err()
}

var mcpTools = []map[string]any{
	{"name": "template_guide", "description": "How to write a DdDesignReport template (bands, placeholders, tables, groups, charts), with a tested example, and its JSON Schema. Read this before writing a template.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}}},
	{"name": "check_template", "description": "Check a template: unknown or misspelt properties, and whether it renders with the sample data. Answers ok, or what is wrong.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"template"}, "properties": map[string]any{
			"template": map[string]any{"type": "object", "description": "the template"},
			"data":     map[string]any{"description": "sample data: an object with the rows under \"rows\", or a list of rows"}}}},
	{"name": "render_template", "description": "Render a template with data to pdf, html, xlsx, xls, csv or jrxml, written to a file. html, csv and jrxml come back as text when no path is given.",
		"inputSchema": map[string]any{"type": "object", "required": []string{"template", "format"}, "properties": map[string]any{
			"template": map[string]any{"type": "object"},
			"data":     map[string]any{},
			"format":   map[string]any{"type": "string", "enum": []string{"pdf", "html", "xlsx", "xls", "csv", "jrxml"}},
			"path":     map[string]any{"type": "string", "description": "file to write the report to"}}}},
}

func mcpCall(method string, params json.RawMessage) (any, error) {
	switch method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		json.Unmarshal(params, &p)
		return map[string]any{"protocolVersion": p.ProtocolVersion, "capabilities": map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{"name": "ddreport", "version": "1"}}, nil
	case "ping":
		return map[string]any{}, nil
	case "tools/list":
		return map[string]any{"tools": mcpTools}, nil
	case "tools/call":
		var p struct {
			Name      string `json:"name"`
			Arguments struct {
				Template json.RawMessage `json:"template"`
				Data     json.RawMessage `json:"data"`
				Format   string          `json:"format"`
				Path     string          `json:"path"`
			} `json:"arguments"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, err
		}
		a := p.Arguments
		if a.Data == nil {
			a.Data = json.RawMessage("[]")
		}
		text, err := "", error(nil)
		switch p.Name {
		case "template_guide":
			schema, _ := designer.Files.ReadFile("template.schema.json")
			text = ddreport.Guide + "\n\n## JSON Schema\n\n" + string(schema)
		case "check_template":
			text, err = checkTemplate(a.Template, a.Data)
		case "render_template":
			var doc []byte
			if doc, err = renderSigned(a.Template, a.Data, a.Format); err == nil {
				switch {
				case a.Path != "":
					if err = os.WriteFile(a.Path, doc, 0o644); err == nil {
						text = fmt.Sprintf("wrote %d bytes of %s to %s", len(doc), a.Format, a.Path)
					}
				case a.Format == "html" || a.Format == "csv" || a.Format == "jrxml":
					text = string(doc)
				default:
					err = fmt.Errorf("give a path to write the %s to", a.Format)
				}
			}
		default:
			return nil, fmt.Errorf("no tool %q", p.Name)
		}
		if err != nil {
			text = err.Error()
		}
		return map[string]any{"content": []map[string]any{{"type": "text", "text": text}}, "isError": err != nil}, nil
	}
	return nil, fmt.Errorf("method %q not supported", method)
}

// checkTemplate names unknown properties (the renderer skips them silently), then renders to PDF.
func checkTemplate(template, data json.RawMessage) (string, error) {
	d := json.NewDecoder(bytes.NewReader(template))
	d.DisallowUnknownFields()
	if err := d.Decode(&ddreport.Template{}); err != nil {
		return "", fmt.Errorf("template: %w", err)
	}
	if _, err := ddreport.Render(template, data, "pdf"); err != nil {
		return "", err
	}
	return "ok: the template is valid and renders", nil
}
