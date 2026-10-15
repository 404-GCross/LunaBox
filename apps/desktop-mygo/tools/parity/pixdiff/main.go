// pixdiff 比较两张截图的同一区域：输出亮度差超过阈值的像素占比，可选输出“原版 | 原型 | 差异×3”的并排图。
//
//	go run ./tools/parity/pixdiff -ref ref.png -mine mygo.png -rect 0,0,1280,240 -scale 1.5 -sbs header.png
//
// -rect 用 DIP（与 dump-styles.js 导出的坐标一致），乘以 -scale 得到截图里的物理像素；省略时比较整张图。
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"strconv"
	"strings"
)

func main() {
	refPath := flag.String("ref", "", "原版截图（PNG）")
	minePath := flag.String("mine", "", "原型截图（PNG）")
	rectArg := flag.String("rect", "", "比较区域 x,y,w,h（DIP）；为空时比较两图的公共部分")
	scale := flag.Float64("scale", 1.5, "截图的设备像素比")
	threshold := flag.Int("threshold", 24, "亮度差（0-255）超过该值的像素计为不同")
	sbsPath := flag.String("sbs", "", "输出并排对比图的路径")
	zoom := flag.Float64("zoom", 1, "并排图的缩放")
	asJSON := flag.Bool("json", false, "以 JSON 输出结果")
	flag.Parse()
	if *refPath == "" || *minePath == "" {
		flag.Usage()
		os.Exit(2)
	}

	ref, mine := load(*refPath), load(*minePath)
	r := ref.Bounds().Intersect(mine.Bounds())
	if *rectArg != "" {
		r = parseRect(*rectArg, *scale).Intersect(r)
	}
	if r.Empty() {
		log.Fatalf("empty region %v (ref %v, mine %v)", r, ref.Bounds(), mine.Bounds())
	}

	diff := image.NewRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	over := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			a, b := luma(ref.At(x, y)), luma(mine.At(x, y))
			d := a - b
			if d < 0 {
				d = -d
			}
			if d > *threshold {
				over++
			}
			v := uint8(min(255, d*3))
			diff.SetRGBA(x-r.Min.X, y-r.Min.Y, color.RGBA{v, v, v, 255})
		}
	}
	total := r.Dx() * r.Dy()
	pct := 100 * float64(over) / float64(total)

	if *sbsPath != "" {
		writeSideBySide(*sbsPath, ref, mine, diff, r, *zoom)
	}
	if *asJSON {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{
			"percent": pct, "over": over, "pixels": total,
			"rect":    []int{r.Min.X, r.Min.Y, r.Dx(), r.Dy()},
			"refSize": []int{ref.Bounds().Dx(), ref.Bounds().Dy()}, "mineSize": []int{mine.Bounds().Dx(), mine.Bounds().Dy()},
		})
		return
	}
	fmt.Printf("diff %.2f%% (%d / %d px over %d) in %v\n", pct, over, total, *threshold, r)
	if ref.Bounds() != mine.Bounds() {
		fmt.Printf("note: sizes differ: ref %v, mine %v\n", ref.Bounds().Size(), mine.Bounds().Size())
	}
}

func load(path string) image.Image {
	f, err := os.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatalf("%s: %v", path, err)
	}
	return img
}

func parseRect(s string, scale float64) image.Rectangle {
	parts := strings.Split(s, ",")
	if len(parts) != 4 {
		log.Fatalf("-rect wants x,y,w,h, got %q", s)
	}
	var v [4]float64
	for i, p := range parts {
		f, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			log.Fatalf("-rect: %v", err)
		}
		v[i] = f * scale
	}
	return image.Rect(int(v[0]), int(v[1]), int(v[0]+v[2]), int(v[1]+v[3]))
}

// luma 与 PIL 的 convert("L") 相同：L = R*299/1000 + G*587/1000 + B*114/1000。
func luma(c color.Color) int {
	r, g, b, _ := c.RGBA()
	return int((r>>8)*299+(g>>8)*587+(b>>8)*114) / 1000
}

func writeSideBySide(path string, ref, mine image.Image, diff *image.RGBA, r image.Rectangle, zoom float64) {
	w, h := int(float64(r.Dx())*zoom), int(float64(r.Dy())*zoom)
	const gap = 10
	out := image.NewRGBA(image.Rect(0, 0, 3*w+2*gap, h))
	draw.Draw(out, out.Bounds(), &image.Uniform{color.RGBA{255, 0, 255, 255}}, image.Point{}, draw.Src)
	for i, src := range []image.Image{ref, mine, diff} {
		origin := r.Min
		if i == 2 {
			origin = image.Point{}
		}
		for y := range h {
			for x := range w {
				sx, sy := origin.X+int(float64(x)/zoom), origin.Y+int(float64(y)/zoom)
				out.Set(i*(w+gap)+x, y, src.At(sx, sy))
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, out); err != nil {
		log.Fatal(err)
	}
}
