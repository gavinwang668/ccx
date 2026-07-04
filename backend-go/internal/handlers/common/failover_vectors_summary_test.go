package common

import (
	"strings"
	"testing"
)

func TestVectorsErrorBodySummaryForLogSanitizesDiagnosticFields(t *testing.T) {
	body := []byte(`{"error":{"message":"embedding input secret customer text was rejected","type":"server_error secret customer text","code":"server_error\nsecret customer text","param":"input\nsecret customer text"},"input":"secret customer text"}`)

	got := errorBodySummaryForLog("Vectors", 500, body)

	if got != "status=500 body=omitted" {
		t.Fatalf("summary = %q, want status-only omitted body", got)
	}
	for _, leaked := range []string{"secret customer text", "\n", "message", "embedding input"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("summary leaked %q: %s", leaked, got)
		}
	}
}
