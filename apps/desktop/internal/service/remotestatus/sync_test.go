package remotestatus

import (
	"context"
	"testing"

	"lunabox/internal/common/enums"
	"lunabox/internal/common/vo"
)

func TestSyncAllReportsDatabaseInitializationFailure(t *testing.T) {
	events := make([]vo.RemoteStatusSyncProgress, 0, 1)
	progress, err := SyncAll(Options{
		Context: context.Background(),
		Source:  enums.Bangumi,
		Emit: func(event vo.RemoteStatusSyncProgress) {
			events = append(events, event)
		},
	})
	if err == nil {
		t.Fatal("expected database initialization error")
	}
	if progress.Status != "failed" || progress.Provider != string(enums.Bangumi) {
		t.Fatalf("unexpected failure progress: %+v", progress)
	}
	if len(events) != 1 || events[0].Status != "failed" {
		t.Fatalf("unexpected emitted events: %+v", events)
	}
}
