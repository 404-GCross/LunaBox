package imageutils

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	webpencoder "github.com/gen2brain/webp"
)

func TestOptimizeCoverImageBytesSkipsSmallWebP(t *testing.T) {
	var input bytes.Buffer
	img := solidImage(320, 480)
	if err := webpencoder.Encode(&input, img, webpencoder.Options{Quality: 82, Method: 4}); err != nil {
		t.Fatalf("encode input webp: %v", err)
	}

	_, ok, err := optimizeCoverImageBytes(input.Bytes(), ".webp")
	if err != nil {
		t.Fatalf("optimizeCoverImageBytes() error = %v", err)
	}
	if ok {
		t.Fatal("optimizeCoverImageBytes() optimized small webp, want skip")
	}
}

func TestOptimizeCoverImageBytesConvertsJPGToWebP(t *testing.T) {
	var input bytes.Buffer
	img := detailedImage(320, 480)
	if err := jpeg.Encode(&input, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode input jpeg: %v", err)
	}

	optimized, ok, err := optimizeCoverImageBytes(input.Bytes(), ".jpg")
	if err != nil {
		t.Fatalf("optimizeCoverImageBytes() error = %v", err)
	}
	if !ok {
		t.Fatal("optimizeCoverImageBytes() skipped jpg, want optimized")
	}
	if optimized.Ext != ".webp" {
		t.Fatalf("optimized.Ext = %q, want .webp", optimized.Ext)
	}
	if optimized.ContentType != "image/webp" {
		t.Fatalf("optimized.ContentType = %q, want image/webp", optimized.ContentType)
	}
	if len(optimized.Data) == 0 {
		t.Fatal("optimized.Data is empty")
	}
}

func TestOptimizeCoverImageBytesResizesLargeImage(t *testing.T) {
	var input bytes.Buffer
	img := detailedImage(2200, 1100)
	if err := jpeg.Encode(&input, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode input jpeg: %v", err)
	}

	optimized, ok, err := optimizeCoverImageBytes(input.Bytes(), ".jpg")
	if err != nil {
		t.Fatalf("optimizeCoverImageBytes() error = %v", err)
	}
	if !ok {
		t.Fatal("optimizeCoverImageBytes() skipped large image, want optimized")
	}

	cfg, err := webpencoder.DecodeConfig(bytes.NewReader(optimized.Data))
	if err != nil {
		t.Fatalf("decode optimized webp config: %v", err)
	}
	if cfg.Width != coverOptimizeMaxSide || cfg.Height != 800 {
		t.Fatalf("optimized size = %dx%d, want %dx800", cfg.Width, cfg.Height, coverOptimizeMaxSide)
	}
}

func solidImage(width, height int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x % 255), G: uint8(y % 255), B: 128, A: 255})
		}
	}
	return img
}

func detailedImage(width, height int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{
				R: uint8((x*13 + y*7) % 255),
				G: uint8((x*x + y*3) % 255),
				B: uint8((x*5 + y*y) % 255),
				A: 255,
			})
		}
	}
	return img
}
