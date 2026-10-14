package test

import (
	"context"
	"lunabox/internal/appconf"
	"lunabox/internal/service"
	"testing"
	"time"
)

func TestTagService_FilterExistingTagNames(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	tagService := service.NewTagService()
	tagService.Init(context.Background(), db, &appconf.AppConfig{})

	now := time.Now()
	for _, item := range []struct {
		id     string
		gameID string
		name   string
	}{
		{id: "tag-1", gameID: "game-1", name: "Love Triangle"},
		{id: "tag-2", gameID: "game-2", name: "Romantic Comedy"},
	} {
		if _, err := db.Exec(
			`INSERT INTO game_tags (id, game_id, name, source, weight, is_spoiler, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			item.id, item.gameID, item.name, "vndb", 1.0, false, now, now,
		); err != nil {
			t.Fatalf("插入 tag 失败: %v", err)
		}
	}

	result, err := tagService.FilterExistingTagNames([]string{
		"",
		"Missing Tag",
		"Love Triangle",
		"Love Triangle",
		"  Romantic Comedy  ",
	})
	if err != nil {
		t.Fatalf("过滤已存在 tag 失败: %v", err)
	}

	expected := []string{"Love Triangle", "Romantic Comedy"}
	if len(result) != len(expected) {
		t.Fatalf("期望 %d 个 tag，实际 %d: %#v", len(expected), len(result), result)
	}
	for index, name := range expected {
		if result[index] != name {
			t.Fatalf("第 %d 个 tag 不匹配，期望 %q，实际 %q", index, name, result[index])
		}
	}
}
