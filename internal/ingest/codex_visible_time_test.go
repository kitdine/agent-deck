package ingest

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"
)

func TestCJSIVisibleDocumentTime(t *testing.T) {
	for _, tc := range []struct{ name, outer, payload, want, proof string }{
		{"outer wins", "2026-01-01T03:04:05.12+02:00", "2026-01-01T12:00:00Z", "2026-01-01T01:04:05.12Z", "2026-01-01T01:04:05.12Z"},
		{"payload only", "", "2026-01-01T05:00:00+02:00", "2026-01-01T03:00:00Z", ""},
		{"invalid outer", "unparseable", "2026-01-01T05:00:00+02:00", "2026-01-01T03:00:00Z", ""},
		{"both absent", "", "", "", ""},
		{"both invalid", "invalid", "invalid", "", ""},
		{"explicit year one", "0001-01-01T00:00:00Z", "2026-01-01T00:00:00Z", "0001-01-01T00:00:00Z", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := map[string]any{"type": "message", "role": "user", "content": "approved"}
			row := map[string]any{"type": "response_item", "payload": p}
			if tc.outer != "" {
				row["timestamp"] = tc.outer
			}
			if tc.payload != "" {
				p["timestamp"] = tc.payload
			}
			buf, err := json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			span := CodexSpan{End: int64(len(buf))}
			proof, visible := lexCodex(buf, span, true)
			if visible == nil || visible.EventAt != tc.want || visible.Text != "approved" {
				t.Fatalf("visible=%+v, want time %q", visible, tc.want)
			}
			only, hidden := lexCodex(buf, span, false)
			if hidden != nil || !reflect.DeepEqual(only, proof) {
				t.Fatal("requesting document material changed shared proof")
			}
			wantProof := time.Time{}
			if tc.proof != "" {
				wantProof, err = time.Parse(time.RFC3339Nano, tc.proof)
				if err != nil {
					t.Fatal(err)
				}
			}
			if !proof.At.Equal(wantProof) {
				t.Fatalf("shared record time %v want %v", proof.At, wantProof)
			}
			p["role"] = "assistant"
			p["phase"] = "commentary"
			buf, err = json.Marshal(row)
			if err != nil {
				t.Fatal(err)
			}
			if _, v := lexCodex(buf, CodexSpan{End: int64(len(buf))}, true); v != nil {
				t.Fatalf("non-final message exposed %+v", v)
			}
		})
	}
}
