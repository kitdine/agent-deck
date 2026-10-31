package telemetry

import (
	"context"
	"testing"
	"time"
)

func TestIncompleteTokenComponentsRemainUnknownAndUncounted(t *testing.T) {
	for _, missing := range []string{"input_token_count", "output_token_count", "both"} {
		t.Run(missing, func(t *testing.T) {
			database, _ := openStore(t)
			fields := completed("partial-"+missing, "1900", "100")
			filtered := fields[:0]
			for _, field := range fields {
				if field.key == missing || (missing == "both" && (field.key == "input_token_count" || field.key == "output_token_count")) {
					continue
				}
				filtered = append(filtered, field)
			}
			filtered = append(filtered, i("total_token_count", "2000"))
			records, err := Decode([]byte(exportBody(codexResource, filtered)))
			if err != nil {
				t.Fatal(err)
			}
			observation := Observation(records[0], "import", receivedAt)
			if records[0].Metered() || observation.Metered || observation.State != "capture_only" {
				t.Fatalf("partial usage is eligible for counting: %+v", observation)
			}
			if missing != "output_token_count" && observation.InputTokens != nil {
				t.Fatal("missing input became zero")
			}
			if missing != "input_token_count" && observation.OutputTokens != nil {
				t.Fatal("missing output became zero")
			}
			if _, err := Import(context.Background(), database, records, receivedAt); err != nil {
				t.Fatal(err)
			}
			recordInventory(t, database, "after", receivedAt.Add(time.Minute))
			settle(t, database, receivedAt.Add(time.Hour))
			assertOwnership(t, database, "partial-"+missing, ownership{states: map[string]int{"capture_only": 1}})
		})
	}
}

func TestSettlementRetractsLegacyIncompleteContributions(t *testing.T) {
	database, _ := openStore(t)
	ownedConversation(t, database, "legacy-partial")
	if _, err := database.DB.Exec(`UPDATE telemetry_observations SET input_tokens=NULL WHERE thread_id='legacy-partial'`); err != nil {
		t.Fatal(err)
	}
	result := settle(t, database, receivedAt.Add(time.Hour))
	if !result.Changed || result.Retracted != 1 {
		t.Fatalf("legacy contribution not retracted: %+v", result)
	}
	assertOwnership(t, database, "legacy-partial", ownership{states: map[string]int{"capture_only": 1}})
	if result := settle(t, database, receivedAt.Add(2*time.Hour)); result.Changed {
		t.Fatalf("repeated demotion changed counted data: %+v", result)
	}
}

func TestTelemetrySettlementAndRetractionPreserveCompletedFileCoverage(t *testing.T) {
	ctx := context.Background()
	database, _ := openStore(t)
	importRecords(t, database, exportBody(codexResource, completed("coverage-background", "1000", "10")))
	generation, err := database.DerivedSnapshotGeneration(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.AcceptInventory(ctx, "complete-files", generation.Epoch, nil, receivedAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := database.FinishInventory(ctx, "complete-files", "complete"); err != nil {
		t.Fatal(err)
	}
	for _, conflict := range []bool{false, true} {
		if conflict {
			importRecords(t, database, exportBody(codexResource, completed("coverage-background", "1000", "11")))
		} else {
			settle(t, database, receivedAt.Add(time.Hour))
		}
		coverage, err := database.ReadSourceCoverage(ctx)
		if err != nil || coverage.State != "complete" {
			t.Fatalf("OTel mutation changed file coverage: %+v %v", coverage, err)
		}
	}
	insertJSONLEvent(t, database, "actual-file", "actual-file-event")
	settle(t, database, receivedAt.Add(2*time.Hour))
	coverage, err := database.ReadSourceCoverage(ctx)
	if err != nil || coverage.State != "update_pending" {
		t.Fatalf("OTel settle hid real JSONL changes: %+v %v", coverage, err)
	}
}

func TestExplicitZeroTokenComponentsRemainCountable(t *testing.T) {
	database, _ := openStore(t)
	importRecords(t, database, exportBody(codexResource, completed("measured-zero", "0", "0")))
	recordInventory(t, database, "after-zero", receivedAt.Add(time.Minute))
	settle(t, database, receivedAt.Add(time.Hour))
	assertOwnership(t, database, "measured-zero", ownership{otelEvents: 1, usageSessions: 1, markers: 1, states: map[string]int{"otel_owned": 1}})
}
