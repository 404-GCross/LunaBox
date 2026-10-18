package covers

import (
	"fmt"
	"image"
	"testing"

	"github.com/egoist/mygo/ui"

	"lunabox/backend/models"
)

// 每张 16MB，7 张超出 budget（96MB）一张。
func addCovers(cc *Cache, n int) []*models.Game {
	games := make([]*models.Game, n)
	for i := range games {
		games[i] = &models.Game{CoverURL: fmt.Sprintf("/local/covers/%d.jpg", i)}
		cc.finish(result{key: games[i].CoverURL, bitmap: ui.NewBitmap(image.NewRGBA(image.Rect(0, 0, 2048, 2048)))})
	}
	return games
}

func TestEndFrameKeepsCoversUsedThisFrame(t *testing.T) {
	cc := New()
	games := addCovers(cc, 7)
	cc.EndFrame()
	if n, _ := cc.Cached(); n != 7 {
		t.Fatalf("cached %d covers, want all 7: they were added this frame", n)
	}
	for _, g := range games {
		if cc.Get(g) == nil {
			t.Fatalf("%s evicted", g.CoverURL)
		}
	}
}

func TestEndFrameEvictsLeastRecentlyUsed(t *testing.T) {
	cc := New()
	games := addCovers(cc, 7)
	cc.EndFrame()
	cc.Get(games[0]) // 最早加入，但这一帧用过
	cc.EndFrame()
	if n, bytes := cc.Cached(); n != 6 || bytes > budget {
		t.Fatalf("cached %d covers (%d bytes), want 6 within budget", n, bytes)
	}
	if cc.Get(games[0]) == nil {
		t.Fatal("cover used last frame was evicted")
	}
	if cc.Get(games[1]) != nil {
		t.Fatal("least recently used cover was kept")
	}
}

func TestForgetReleasesBytes(t *testing.T) {
	cc := New()
	games := addCovers(cc, 2)
	cc.Forget(games[0])
	if n, bytes := cc.Cached(); n != 1 || bytes != 4*2048*2048 {
		t.Fatalf("cached %d covers (%d bytes) after Forget", n, bytes)
	}
}
