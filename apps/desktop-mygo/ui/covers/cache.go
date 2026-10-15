// Package covers 在后台读取、解码、缩放游戏封面，对应原版的 GameCoverImage。
package covers

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
	_ "golang.org/x/image/webp"

	"lunabox/desktop-mygo/ui/assets"
	"lunabox/internal/models"
)

// maxWidth 是解码后保留的最大宽度（像素）。卡片约 150 DIP，200% 缩放下 300px 足够。
const maxWidth = 360

type request struct {
	key      string
	fallback string
}

type result struct {
	key    string
	bitmap *ui.Bitmap
	err    error
	took   time.Duration
}

// Cache 在后台 goroutine 中读取、解码、缩放封面，结果通过 Window.Update 回到主线程。
// bitmaps/pending/failed 只在主线程读写。对应 GameCoverImage：cover_url 失败时回退 cover_source_url。
type Cache struct {
	dataDir string
	win     *mygo.Window
	queue   chan request
	client  *http.Client

	bitmaps map[string]*ui.Bitmap
	pending map[string]bool
	failed  map[string]bool

	decodeTotal  time.Duration
	decoded      int
	releaseTimer *time.Timer
}

func New() *Cache {
	return &Cache{
		queue:   make(chan request, 512),
		client:  &http.Client{Timeout: 15 * time.Second},
		bitmaps: make(map[string]*ui.Bitmap),
		pending: make(map[string]bool),
		failed:  make(map[string]bool),
	}
}

// Start 在窗口创建后启动解码 goroutine；dataDir 用于解析 /local/ 开头的封面地址。
func (cc *Cache) Start(win *mygo.Window, dataDir string) {
	cc.win = win
	cc.dataDir = dataDir
	for range max(2, runtime.NumCPU()/2) {
		go cc.worker()
	}
}

// key 返回游戏用于显示的封面地址；没有封面时为空。
func key(g *models.Game) (key, fallback string) {
	key, fallback = g.CoverURL, g.CoverSourceURL
	if key == "" {
		key, fallback = fallback, ""
	}
	return key, fallback
}

// Get 返回已解码的封面；未就绪时排队加载并返回 nil。只在主线程调用。
func (cc *Cache) Get(g *models.Game) *ui.Bitmap {
	k, fallback := key(g)
	if k == "" || cc.win == nil {
		return nil
	}
	if b, ok := cc.bitmaps[k]; ok {
		return b
	}
	if cc.pending[k] || cc.failed[k] {
		return nil
	}
	select {
	case cc.queue <- request{key: k, fallback: fallback}:
		cc.pending[k] = true
	default:
		// 队列已满，下一帧再试。
	}
	return nil
}

func (cc *Cache) worker() {
	for req := range cc.queue {
		start := time.Now()
		bmp, err := cc.load(req.key)
		if err != nil && req.fallback != "" {
			bmp, err = cc.load(req.fallback)
		}
		res := result{key: req.key, bitmap: bmp, err: err, took: time.Since(start)}
		cc.win.Update(func() { cc.finish(res) })
	}
}

func (cc *Cache) finish(res result) {
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
func (cc *Cache) scheduleRelease() {
	if cc.releaseTimer != nil {
		cc.releaseTimer.Reset(time.Second)
		return
	}
	cc.releaseTimer = time.AfterFunc(time.Second, debug.FreeOSMemory)
}

func (cc *Cache) load(src string) (*ui.Bitmap, error) {
	data, err := cc.read(src)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", src, err)
	}
	return ui.NewBitmap(assets.Downscale(img, maxWidth)), nil
}

// read 解析主程序使用的封面地址：/local/... 指向数据目录，http(s) 走网络。
func (cc *Cache) read(src string) ([]byte, error) {
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

// LoadFull 读取并解码大图（图片查看器用），最宽 maxWidth 像素；cover_url 失败时回退 cover_source_url。
// 会阻塞，在后台 goroutine 里调用；结果不进缓存。
func (cc *Cache) LoadFull(g *models.Game, maxWidth int) (*ui.Bitmap, error) {
	k, fallback := key(g)
	if k == "" {
		return nil, fmt.Errorf("game %s has no cover", g.ID)
	}
	load := func(src string) (*ui.Bitmap, error) {
		data, err := cc.read(src)
		if err != nil {
			return nil, err
		}
		img, _, err := image.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", src, err)
		}
		return ui.NewBitmap(assets.Downscale(img, maxWidth)), nil
	}
	bmp, err := load(k)
	if err != nil && fallback != "" {
		bmp, err = load(fallback)
	}
	return bmp, err
}

// Forget 丢掉游戏封面的缓存，下次 Get 时重新读取。本地封面换图后地址不变（/local/covers/<id>.jpg），
// 所以换封面（选择图片、下载远程封面、cover-image:download 完成）之后要调用它。只在主线程调用。
func (cc *Cache) Forget(g *models.Game) {
	for _, k := range []string{g.CoverURL, g.CoverSourceURL} {
		if k != "" && !cc.pending[k] {
			delete(cc.bitmaps, k)
			delete(cc.failed, k)
		}
	}
}

// Missing 报告游戏没有封面，或封面加载失败（显示 i-mdi-image-off）。
func (cc *Cache) Missing(g *models.Game) bool {
	k, _ := key(g)
	return k == "" || cc.failed[k]
}

// Pending 是排队或解码中的封面数。
func (cc *Cache) Pending() int { return len(cc.pending) }

// Decoded 返回已解码的封面数与平均耗时。
func (cc *Cache) Decoded() (n int, avg time.Duration) {
	if cc.decoded == 0 {
		return 0, 0
	}
	return cc.decoded, cc.decodeTotal / time.Duration(cc.decoded)
}
