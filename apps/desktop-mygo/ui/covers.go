package ui

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"

	"lunabox/internal/models"
)

// coverMaxWidth 是解码后保留的最大宽度（像素）。卡片约 150 DIP，200% 缩放下 300px 足够。
const coverMaxWidth = 360

type coverRequest struct {
	key      string
	fallback string
}

type coverResult struct {
	key    string
	bitmap *ui.Bitmap
	err    error
	took   time.Duration
}

// coverCache 在后台 goroutine 中读取、解码、缩放封面，结果通过 Window.Update 回到主线程。
// bitmaps/pending/failed 只在主线程读写。对应 GameCoverImage：cover_url 失败时回退 cover_source_url。
type coverCache struct {
	dataDir string
	win     *mygo.Window
	queue   chan coverRequest
	client  *http.Client

	bitmaps map[string]*ui.Bitmap
	pending map[string]bool
	failed  map[string]bool

	decodeTotal  time.Duration
	decoded      int
	releaseTimer *time.Timer
}

func newCoverCache() *coverCache {
	return &coverCache{
		queue:   make(chan coverRequest, 512),
		client:  &http.Client{Timeout: 15 * time.Second},
		bitmaps: make(map[string]*ui.Bitmap),
		pending: make(map[string]bool),
		failed:  make(map[string]bool),
	}
}

func (cc *coverCache) start(win *mygo.Window, dataDir string) {
	cc.win = win
	cc.dataDir = dataDir
	for range max(2, runtime.NumCPU()/2) {
		go cc.worker()
	}
}

// key 返回游戏用于显示的封面地址；没有封面时为空。
func coverKey(g *models.Game) (key, fallback string) {
	key, fallback = g.CoverURL, g.CoverSourceURL
	if key == "" {
		key, fallback = fallback, ""
	}
	return key, fallback
}

// get 返回已解码的封面；未就绪时排队加载并返回 nil。只在主线程调用。
func (cc *coverCache) get(g *models.Game) *ui.Bitmap {
	key, fallback := coverKey(g)
	if key == "" || cc.win == nil {
		return nil
	}
	if b, ok := cc.bitmaps[key]; ok {
		return b
	}
	if cc.pending[key] || cc.failed[key] {
		return nil
	}
	select {
	case cc.queue <- coverRequest{key: key, fallback: fallback}:
		cc.pending[key] = true
	default:
		// 队列已满，下一帧再试。
	}
	return nil
}

func (cc *coverCache) worker() {
	for req := range cc.queue {
		start := time.Now()
		bmp, err := cc.load(req.key)
		if err != nil && req.fallback != "" {
			bmp, err = cc.load(req.fallback)
		}
		res := coverResult{key: req.key, bitmap: bmp, err: err, took: time.Since(start)}
		cc.win.Update(func() { cc.finish(res) })
	}
}

func (cc *coverCache) finish(res coverResult) {
	delete(cc.pending, res.key)
	if res.err != nil {
		cc.failed[res.key] = true
		return
	}
	cc.bitmaps[res.key] = res.bitmap
	cc.decoded++
	cc.decodeTotal += res.took
	if len(cc.pending) == 0 {
		cc.scheduleRelease()
	}
}

// scheduleRelease 在一批封面解码完后把全尺寸解码留下的垃圾还给系统。
// 不这样做的话 GOGC=100 会让堆在两次 GC 之间涨到常驻量的两倍，内存对比会失真。
func (cc *coverCache) scheduleRelease() {
	if cc.releaseTimer != nil {
		cc.releaseTimer.Reset(time.Second)
		return
	}
	cc.releaseTimer = time.AfterFunc(time.Second, debug.FreeOSMemory)
}

func (cc *coverCache) load(src string) (*ui.Bitmap, error) {
	data, err := cc.read(src)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", src, err)
	}
	return ui.NewBitmap(downscale(img, coverMaxWidth)), nil
}

// read 解析主程序使用的封面地址：/local/... 指向数据目录，http(s) 走网络。
func (cc *coverCache) read(src string) ([]byte, error) {
	if i := strings.Index(src, "/local/"); i >= 0 {
		rel := strings.TrimPrefix(src[i:], "/local/")
		if j := strings.IndexAny(rel, "?#"); j >= 0 {
			rel = rel[:j]
		}
		return os.ReadFile(filepath.Join(cc.dataDir, filepath.FromSlash(rel)))
	}
	if strings.HasPrefix(src, "http://") || strings.HasPrefix(src, "https://") {
		resp, err := cc.client.Get(src)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("fetch %s: %s", src, resp.Status)
		}
		return io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	}
	return os.ReadFile(src)
}

func scaleBilinear(dst *image.RGBA, src image.Image) {
	draw.BiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
}
