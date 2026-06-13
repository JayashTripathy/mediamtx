package recorder

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/stretchr/testify/require"
)

func jpegImage(t *testing.T, fill color.Color) []byte {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.Set(x, y, fill)
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, nil)
	require.NoError(t, err)
	return buf.Bytes()
}

func TestMotionDetectorDetectJPEG(t *testing.T) {
	detector := &motionDetector{
		threshold: 10,
		minPixels: 4,
	}

	motion, err := detector.detectJPEG(jpegImage(t, color.RGBA{10, 10, 10, 255}))
	require.NoError(t, err)
	require.False(t, motion)

	motion, err = detector.detectJPEG(jpegImage(t, color.RGBA{12, 12, 12, 255}))
	require.NoError(t, err)
	require.False(t, motion)

	motion, err = detector.detectJPEG(jpegImage(t, color.RGBA{80, 80, 80, 255}))
	require.NoError(t, err)
	require.True(t, motion)
}
