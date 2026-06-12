package tree

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrintReusedActionsJSON(t *testing.T) {
	var buf bytes.Buffer
	uses := []string{"actions/checkout@v4", "actions/setup-node@v4"}
	if err := PrintReusedActionsJSON(uses, &buf); err != nil {
		t.Fatalf("PrintReusedActionsJSON: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, `"uses": "actions/checkout@v4"`) {
		t.Errorf("missing checkout entry: %s", got)
	}
	if !strings.Contains(got, `"uses": "actions/setup-node@v4"`) {
		t.Errorf("missing setup-node entry: %s", got)
	}
	if strings.Contains(got, "contributors") || strings.Contains(got, "stars") || strings.Contains(got, "released_on") {
		t.Errorf("output should not contain metadata fields: %s", got)
	}
}
