package tree

import (
	"bytes"
	"embed"
	"encoding/json"
	"html/template"
	"os"

	"github.com/sparkrew/rechta/resolver"
)

//go:embed html/template.html
var htmlTemplateFS embed.FS

var htmlTmpl = template.Must(template.ParseFS(htmlTemplateFS, "html/template.html"))

// PrintHTML writes a self-contained interactive HTML dependency report to path.
func PrintHTML(trees []resolver.WorkflowTree, path string) error {
	payload, err := json.Marshal(struct {
		Workflows []resolver.WorkflowTree `json:"workflows"`
	}{Workflows: trees})
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := htmlTmpl.Execute(&buf, struct {
		JSON template.JS
	}{JSON: template.JS(payload)}); err != nil {
		return err
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}
