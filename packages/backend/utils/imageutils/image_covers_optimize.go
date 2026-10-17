package imageutils

import (
	"bytes"
	"fmt"
	"image"
	"strings"

	webpencoder "github.com/gen2brain/webp"
	"golang.org/x/image/draw"
)

const (
	coverOptimizeMaxSide      = 1600
	coverOptimizeMinBytes     = 512 * 1024
	coverOptimizeWebPQuality  = 82
	coverOptimizeWebPMethod   = 4
	optimizedCoverExtension   = ".webp"
	optimizedCoverContentType = "image/webp"
)

type optimizedCoverImage struct {
	Data        []byte
	Ext         string
	ContentType string
}

func optimizeCoverImageBytes(data []byte, sourceExt string) (optimizedCoverImage, bool, error) {
	sourceExt = strings.ToLower(sourceExt)
	if len(data) == 0 || sourceExt == ".avif" || sourceExt == ".gif" {
		return optimizedCoverImage{}, false, nil
	}

	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return optimizedCoverImage{}, false, nil
	}
	if format == "gif" {
		return optimizedCoverImage{}, false, nil
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	shouldOptimize := sourceExt != optimizedCoverExtension ||
		len(data) > coverOptimizeMinBytes ||
		width > coverOptimizeMaxSide ||
		height > coverOptimizeMaxSide
	if !shouldOptimize {
		return optimizedCoverImage{}, false, nil
	}

	output := img
	if width > coverOptimizeMaxSide || height > coverOptimizeMaxSide {
		output = resizeImageToMaxSide(img, coverOptimizeMaxSide)
	}

	var buf bytes.Buffer
	if err := webpencoder.Encode(&buf, output, webpencoder.Options{
		Quality: coverOptimizeWebPQuality,
		Method:  coverOptimizeWebPMethod,
	}); err != nil {
		return optimizedCoverImage{}, false, fmt.Errorf("encode optimized cover image: %w", err)
	}

	if buf.Len() >= len(data) {
		return optimizedCoverImage{}, false, nil
	}

	return optimizedCoverImage{
		Data:        buf.Bytes(),
		Ext:         optimizedCoverExtension,
		ContentType: optimizedCoverContentType,
	}, true, nil
}

func resizeImageToMaxSide(src image.Image, maxSide int) image.Image {
	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= maxSide && height <= maxSide {
		return src
	}

	newWidth := maxSide
	newHeight := maxSide
	if width >= height {
		newHeight = max(1, height*maxSide/width)
	} else {
		newWidth = max(1, width*maxSide/height)
	}

	dst := image.NewNRGBA(image.Rect(0, 0, newWidth, newHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	return dst
}
