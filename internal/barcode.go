package internal

import (
	"image"
	_ "image/jpeg"
	"os"
	"strings"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/oned"
)

func ReadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

func ParseBarcode(image image.Image) (string, error) {
	bitmap, err := gozxing.NewBinaryBitmapFromImage(image)
	if err != nil {
		return "", err
	}

	reader := oned.NewMultiFormatUPCEANReader(nil)
	result, err := reader.Decode(bitmap, nil)
	if err != nil {
		return "", err
	}

	code, _ := strings.CutPrefix(result.GetText(), "0")
	return code, nil
}

func GenerateBarcode(code string) (*gozxing.BitMatrix, error) {
	writer := oned.NewCode128Writer()
	bmp, err := writer.Encode(code, gozxing.BarcodeFormat_CODE_128, 400, 200, nil)
	if err != nil {
		return nil, err
	}

	return bmp, nil
}
