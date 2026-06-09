package recorder

import (
	"bytes"
	"image"
	"image/jpeg"
)

type motionDetector struct {
	threshold uint8
	minPixels int

	prev   []uint8
	width  int
	height int
}

func (d *motionDetector) detectJPEG(payload []byte) (bool, error) {
	img, err := jpeg.Decode(bytes.NewReader(payload))
	if err != nil {
		return false, err
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	cur := grayscale(img, bounds)

	if d.prev == nil || d.width != width || d.height != height {
		d.prev = cur
		d.width = width
		d.height = height
		return false, nil
	}

	changed := 0
	threshold := int(d.threshold)
	for i, v := range cur {
		diff := int(v) - int(d.prev[i])
		if diff < 0 {
			diff = -diff
		}
		if diff >= threshold {
			changed++
			if changed >= d.minPixels {
				d.prev = cur
				return true, nil
			}
		}
	}

	d.prev = cur
	return false, nil
}

func grayscale(img image.Image, bounds image.Rectangle) []uint8 {
	out := make([]uint8, 0, bounds.Dx()*bounds.Dy())
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			out = append(out, uint8((299*r+587*g+114*b+500000)/1000000))
		}
	}
	return out
}
