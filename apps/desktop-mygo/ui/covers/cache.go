// Package covers 在后台读取、解码、缩放游戏封面，对应原版的 GameCoverImage。
package covers

import (
	"bytes"
	"container/list"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	_ "github.com/gen2brain/avif"
	_ "golang.org/x/image/webp"

	"lunabox/backend/models"
	"lunabox/desktop-mygo/ui/assets"
)

// maxWidth 是解码后保留的最大宽度（像素）。卡片约 150 DIP，200% 缩放下 300px 足够。
const maxWidth = 360

// largeWidth、largeHeight 是首页大图（Large）保留的最大尺寸：大图最大 512×288 DIP，200% 缩放下 1024×576px。
const (
	largeWidth  = 1024
	largeHeight = 576
)

// largeSuffix 区分同一地址的大图与普通封面。
const largeSuffix = "#large"

// readers 是同时读取封面文件的 goroutine 数：远程封面可能要等十几秒，读取与解码分开，慢的下载不占解码名额。
const readers = 6

// decodeSlots 是同时解码的封面数。解码要先得到整张原图（大的有 10MP、40MB），并发越多峰值内存越高。
const decodeSlots = 3

// budget 是已解码封面占用内存的上限（字节），约 160 张卡片封面（360px 宽，每张约 0.6MB）。
const budget = 96 << 20

type request struct {
	key      string // 缓存的键：封面地址，大图再加 largeSuffix
	src      string
	fallback string
	width    int
	height   int  // 不为 0 时同时限制高度
	accent   bool // 同时计算主色（Accent）
}

type result struct {
	key       string
	bitmap    *ui.Bitmap
	accent    ui.Color
	hasAccent bool
	err       error
	took      time.Duration
}

// entry 是 LRU 中的一张已解码封面。
type entry struct {
	key   string
	bmp   *ui.Bitmap
	bytes int
	frame uint64 // 最后一次用到它的帧
}

// Cache 在后台 goroutine 中读取、解码、缩放封面，结果通过 Window.Update 回到主线程。
// bitmaps/lru/pending/failed 只在主线程读写。对应 GameCoverImage：cover_url 失败时回退 cover_source_url。
// 已解码的封面按最近使用（LRU）保留，总量超出 budget 时由 EndFrame 丢弃最久未用的。
type Cache struct {
	dataDir string
	win     *mygo.Window
	queue   chan request
	slots   chan struct{} // 解码名额，见 decodeSlots
	client  *http.Client

	bitmaps map[string]*list.Element // 值是 *entry
	lru     *list.List               // 最近用到的在前
	bytes   int
	frame   uint64
	accents map[string]ui.Color // 大图的主色，键是封面地址
	pending map[string]bool
	failed  map[string]bool

	bg background // 根布局的背景图（Background）

	decodeTotal  time.Duration
	decoded      int
	releaseTimer *time.Timer
}

func New() *Cache {
	return &Cache{
		queue:   make(chan request, 512),
		slots:   make(chan struct{}, decodeSlots),
		client:  &http.Client{Timeout: 15 * time.Second},
		bitmaps: make(map[string]*list.Element),
		lru:     list.New(),
		accents: make(map[string]ui.Color),
		pending: make(map[string]bool),
		failed:  make(map[string]bool),
	}
}

// Start 在窗口创建后启动读取 goroutine；dataDir 用于解析 /local/ 开头的封面地址。
func (cc *Cache) Start(win *mygo.Window, dataDir string) {
	cc.win = win
	cc.dataDir = dataDir
	for range readers {
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
	return cc.get(request{key: k, src: k, fallback: fallback, width: maxWidth})
}

// Large 返回首页大图用的封面（最宽 largeWidth 像素），并计算主色（Accent）；未就绪时排队加载并返回 nil。只在主线程调用。
func (cc *Cache) Large(g *models.Game) *ui.Bitmap {
	k, fallback := key(g)
	return cc.get(request{key: k + largeSuffix, src: k, fallback: fallback, width: largeWidth, height: largeHeight, accent: true})
}

// LargeMissing 报告游戏没有封面，或大图加载失败。
func (cc *Cache) LargeMissing(g *models.Game) bool {
	k, _ := key(g)
	return k == "" || cc.failed[k+largeSuffix]
}

// Accent 对应 useImageAccentRgb：大图（Large）的主色，大图还没加载完或加载失败时 ok 为 false。
func (cc *Cache) Accent(g *models.Game) (c ui.Color, ok bool) {
	k, _ := key(g)
	c, ok = cc.accents[k]
	return c, ok
}

func (cc *Cache) get(req request) *ui.Bitmap {
	if req.src == "" {
		return nil
	}
	k := req.key
	if el, ok := cc.bitmaps[k]; ok {
		e := el.Value.(*entry)
		e.frame = cc.frame
		cc.lru.MoveToFront(el)
		return e.bmp
	}
	if cc.win == nil || cc.pending[k] || cc.failed[k] {
		return nil
	}
	select {
	case cc.queue <- req:
		cc.pending[k] = true
	default:
		// 队列已满，下一帧再试。
	}
	return nil
}

func (cc *Cache) worker() {
	for req := range cc.queue {
		start := time.Now()
		res := result{key: req.key}
		use := func(img image.Image) {
			width := req.width
			if b := img.Bounds(); req.height > 0 && b.Dy() > req.height {
				width = min(width, max(1, b.Dx()*req.height/b.Dy()))
			}
			res.bitmap = ui.NewBitmap(assets.Downscale(img, width))
			if req.accent {
				res.accent, res.hasAccent = accentColor(img), true
			}
		}
		res.err = cc.decode(req.src, use)
		if res.err != nil && req.fallback != "" {
			res.err = cc.decode(req.fallback, use)
		}
		res.took = time.Since(start)
		cc.win.Update(func() { cc.finish(res) })
	}
}

func (cc *Cache) finish(res result) {
	delete(cc.pending, res.key)
	if res.err != nil {
		cc.failed[res.key] = true
	} else {
		w, h := res.bitmap.Size()
		e := &entry{key: res.key, bmp: res.bitmap, bytes: 4 * w * h, frame: cc.frame}
		cc.bitmaps[res.key] = cc.lru.PushFront(e)
		cc.bytes += e.bytes
		if res.hasAccent {
			cc.accents[strings.TrimSuffix(res.key, largeSuffix)] = res.accent
		}
		cc.decoded++
		cc.decodeTotal += res.took
	}
	// 失败的封面也要触发：批次里最后完成的若是失败的（如损坏的图片），否则这一批的解码垃圾一直留在进程里。
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

// EndFrame 在每次构建界面之后调用：已解码的封面超出 budget 时，从最久未用的开始丢弃，
// 但不丢这一帧用到的——窗口很大、一屏的封面就超出预算时宁可暂时超出，也不反复重新解码。只在主线程调用。
func (cc *Cache) EndFrame() {
	for cc.bytes > budget {
		el := cc.lru.Back()
		if el.Value.(*entry).frame == cc.frame {
			break
		}
		cc.remove(el)
	}
	cc.frame++
}

func (cc *Cache) remove(el *list.Element) {
	e := cc.lru.Remove(el).(*entry)
	delete(cc.bitmaps, e.key)
	cc.bytes -= e.bytes
}

// decode 读取 src，在解码名额（decodeSlots）内解码并交给 use。全尺寸原图只在 use 里用，名额归还后即可回收。
func (cc *Cache) decode(src string, use func(image.Image)) error {
	data, err := cc.read(src)
	if err != nil {
		return err
	}
	cc.slots <- struct{}{}
	defer func() { <-cc.slots }()
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("decode %s: %w", src, err)
	}
	use(img)
	return nil
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
	var bmp *ui.Bitmap
	use := func(img image.Image) { bmp = ui.NewBitmap(assets.Downscale(img, maxWidth)) }
	err := cc.decode(k, use)
	if err != nil && fallback != "" {
		err = cc.decode(fallback, use)
	}
	if err != nil {
		return nil, err
	}
	return bmp, nil
}

// Forget 丢掉游戏封面的缓存，下次 Get 时重新读取。本地封面换图后地址不变（/local/covers/<id>.jpg），
// 所以换封面（选择图片、下载远程封面、cover-image:download 完成）之后要调用它。只在主线程调用。
func (cc *Cache) Forget(g *models.Game) {
	for _, k := range []string{g.CoverURL, g.CoverSourceURL, g.CoverURL + largeSuffix, g.CoverSourceURL + largeSuffix} {
		if k != "" && k != largeSuffix && !cc.pending[k] {
			if el, ok := cc.bitmaps[k]; ok {
				cc.remove(el)
			}
			delete(cc.failed, k)
			delete(cc.accents, k)
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

// Cached 返回缓存中的封面数与它们占用的内存（字节）。
func (cc *Cache) Cached() (n, bytes int) { return cc.lru.Len(), cc.bytes }
