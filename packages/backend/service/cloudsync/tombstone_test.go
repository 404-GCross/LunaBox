package cloudsync

import "testing"

func TestTombstoneIDs(t *testing.T) {
	if got := RelationTombstoneID("game", "category"); got != "game::category" {
		t.Fatalf("unexpected relation tombstone id: %q", got)
	}
	if got := TagTombstoneID("game", "user", "tag"); got != "game::user::tag" {
		t.Fatalf("unexpected tag tombstone id: %q", got)
	}
	if got := MetadataSourceTombstoneID("game", "vndb"); got != "game::vndb" {
		t.Fatalf("unexpected metadata source tombstone id: %q", got)
	}
}
