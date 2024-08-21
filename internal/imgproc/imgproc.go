package imgproc

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
)

// ThresholdMode represents the thresholding mode
type ThresholdMode int

const (
	// ThreshBinary uses a fixed threshold value
	ThreshBinary ThresholdMode = iota
	// ThreshOtsu uses Otsu's method to determine the threshold
	ThreshOtsu
)

// ToGrayscale converts an image to grayscale
func ToGrayscale(img image.Image) *image.Gray {
	bounds := img.Bounds()
	grayImg := image.NewGray(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			oldColor := img.At(x, y)
			r, g, b, _ := oldColor.RGBA()
			gray := uint8((0.299*float64(r) + 0.587*float64(g) + 0.114*float64(b)) / 256.0)
			grayImg.Set(x, y, color.Gray{Y: gray})
		}
	}

	return grayImg
}

// Threshold applies binary thresholding to a grayscale image
func Threshold(img *image.Gray, thresh uint8, mode ThresholdMode) *image.Gray {
	bounds := img.Bounds()
	binaryImg := image.NewGray(bounds)

	if mode == ThreshOtsu {
		thresh = otsuThreshold(img)
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if img.GrayAt(x, y).Y > thresh {
				binaryImg.Set(x, y, color.White)
			} else {
				binaryImg.Set(x, y, color.Black)
			}
		}
	}

	return binaryImg
}

// otsuThreshold calculates the optimal threshold using Otsu's method
func otsuThreshold(img *image.Gray) uint8 {
	histogram := make([]int, 256)
	bounds := img.Bounds()
	totalPixels := (bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			histogram[img.GrayAt(x, y).Y]++
		}
	}

	sum := 0
	for i := 0; i < 256; i++ {
		sum += i * histogram[i]
	}

	sumB := 0
	wB := 0
	wF := 0
	varMax := 0.0
	threshold := 0

	for i := 0; i < 256; i++ {
		wB += histogram[i]
		if wB == 0 {
			continue
		}
		wF = totalPixels - wB
		if wF == 0 {
			break
		}
		sumB += i * histogram[i]
		mB := float64(sumB) / float64(wB)
		mF := float64(sum-sumB) / float64(wF)
		varBetween := float64(wB) * float64(wF) * (mB - mF) * (mB - mF)
		if varBetween > varMax {
			varMax = varBetween
			threshold = i
		}
	}

	return uint8(threshold)
}

// ProcessImage applies grayscale and threshold to an image
func ProcessImage(img image.Image, thresh uint8, mode ThresholdMode) *image.Gray {
	grayImg := ToGrayscale(img)
	return Threshold(grayImg, thresh, mode)
}

// DecodeBase64Image decodes a base64 encoded image
func DecodeBase64Image(b64 string) (image.Image, error) {
	reader := base64.NewDecoder(base64.StdEncoding, bytes.NewBufferString(b64))
	img, _, err := image.Decode(reader)
	return img, err
}

// EncodeToBase64 encodes an image to base64
func EncodeToBase64(img image.Image) (string, error) {
	var buf bytes.Buffer
	err := png.Encode(&buf, img)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

// BytesToImage converts a byte slice to an image.Image
func BytesToImage(data []byte) (image.Image, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return img, nil
}

// GrayImageToPNGBytes converts an image.Gray to PNG format byte slice
func GrayImageToPNGBytes(img *image.Gray) ([]byte, error) {
	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}