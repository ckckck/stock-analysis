package main

import (
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsIconMatchesAppIcon(t *testing.T) {
	projectDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}

	appIconPath := filepath.Join(projectDir, "build", "appicon.png")
	iconPath := filepath.Join(projectDir, "build", "windows", "icon.ico")

	appIconFile, err := os.Open(appIconPath)
	if err != nil {
		t.Fatalf("os.Open(%q) error = %v", appIconPath, err)
	}
	defer appIconFile.Close()

	appIcon, err := png.Decode(appIconFile)
	if err != nil {
		t.Fatalf("png.Decode(%q) error = %v", appIconPath, err)
	}

	windowsIcon, err := decodeSingle32BitICO(iconPath)
	if err != nil {
		t.Fatalf("decodeSingle32BitICO(%q) error = %v", iconPath, err)
	}

	expected := boxScale(appIcon, windowsIcon.Bounds().Dx(), windowsIcon.Bounds().Dy())
	meanDelta := meanRGBAAbsDelta(expected, windowsIcon)
	if meanDelta > 20 {
		t.Fatalf("windows icon diverges from appicon: mean RGBA delta = %.2f, want <= 20.00", meanDelta)
	}
}

func decodeSingle32BitICO(path string) (*image.NRGBA, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 22 {
		return nil, os.ErrInvalid
	}
	if binary.LittleEndian.Uint16(data[0:2]) != 0 || binary.LittleEndian.Uint16(data[2:4]) != 1 {
		return nil, os.ErrInvalid
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count < 1 || len(data) < 6+count*16 {
		return nil, os.ErrInvalid
	}

	type iconEntry struct {
		width      int
		height     int
		bitCount   uint16
		bytesInRes int
		offset     int
	}

	bestIndex := -1
	bestDistance := int(^uint(0) >> 1)
	entries := make([]iconEntry, 0, count)
	for i := 0; i < count; i++ {
		entry := data[6+i*16 : 6+(i+1)*16]
		width := int(entry[0])
		if width == 0 {
			width = 256
		}
		height := int(entry[1])
		if height == 0 {
			height = 256
		}
		entries = append(entries, iconEntry{
			width:      width,
			height:     height,
			bitCount:   binary.LittleEndian.Uint16(entry[6:8]),
			bytesInRes: int(binary.LittleEndian.Uint32(entry[8:12])),
			offset:     int(binary.LittleEndian.Uint32(entry[12:16])),
		})

		distance := absInt(width-128) + absInt(height-128)
		if bestIndex == -1 || distance < bestDistance {
			bestIndex = i
			bestDistance = distance
		}
	}
	if bestIndex == -1 {
		return nil, os.ErrInvalid
	}

	selected := entries[bestIndex]
	if selected.offset < 0 || selected.bytesInRes <= 0 || selected.offset+selected.bytesInRes > len(data) {
		return nil, os.ErrInvalid
	}

	payload := data[selected.offset : selected.offset+selected.bytesInRes]
	if len(payload) >= 8 && string(payload[:8]) == "\x89PNG\r\n\x1a\n" {
		img, err := png.Decode(bytesReader(payload))
		if err != nil {
			return nil, err
		}
		return toNRGBA(img), nil
	}

	if selected.bitCount != 32 {
		return nil, os.ErrInvalid
	}

	dib := payload
	if len(dib) < 40 {
		return nil, os.ErrInvalid
	}
	headerSize := int(binary.LittleEndian.Uint32(dib[0:4]))
	if headerSize < 40 || len(dib) < headerSize {
		return nil, os.ErrInvalid
	}
	dibWidth := int(int32(binary.LittleEndian.Uint32(dib[4:8])))
	dibHeight := int(int32(binary.LittleEndian.Uint32(dib[8:12]))) / 2
	if dibWidth != selected.width || dibHeight != selected.height {
		return nil, os.ErrInvalid
	}

	pixels := dib[headerSize:]
	expectedPixels := selected.width * selected.height * 4
	if len(pixels) < expectedPixels {
		return nil, os.ErrInvalid
	}

	img := image.NewNRGBA(image.Rect(0, 0, selected.width, selected.height))
	for y := 0; y < selected.height; y++ {
		srcY := selected.height - 1 - y
		for x := 0; x < selected.width; x++ {
			src := (srcY*selected.width + x) * 4
			dst := img.PixOffset(x, y)
			img.Pix[dst+0] = pixels[src+2]
			img.Pix[dst+1] = pixels[src+1]
			img.Pix[dst+2] = pixels[src+0]
			img.Pix[dst+3] = pixels[src+3]
		}
	}
	return img, nil
}

func bytesReader(data []byte) io.Reader {
	return &sliceReader{data: data}
}

type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func boxScale(src image.Image, width, height int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, width, height))
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()

	for y := 0; y < height; y++ {
		y0 := srcBounds.Min.Y + y*srcHeight/height
		y1 := srcBounds.Min.Y + (y+1)*srcHeight/height
		if y1 <= y0 {
			y1 = y0 + 1
		}
		for x := 0; x < width; x++ {
			x0 := srcBounds.Min.X + x*srcWidth/width
			x1 := srcBounds.Min.X + (x+1)*srcWidth/width
			if x1 <= x0 {
				x1 = x0 + 1
			}
			dst.SetNRGBA(x, y, averageBlock(src, x0, y0, x1, y1))
		}
	}

	return dst
}

func toNRGBA(src image.Image) *image.NRGBA {
	bounds := src.Bounds()
	dst := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			dst.Set(x-bounds.Min.X, y-bounds.Min.Y, src.At(x, y))
		}
	}
	return dst
}

func averageBlock(src image.Image, x0, y0, x1, y1 int) color.NRGBA {
	var sumR, sumG, sumB, sumA uint64
	var count uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			r, g, b, a := src.At(x, y).RGBA()
			sumR += uint64(r >> 8)
			sumG += uint64(g >> 8)
			sumB += uint64(b >> 8)
			sumA += uint64(a >> 8)
			count++
		}
	}

	return color.NRGBA{
		R: uint8(sumR / count),
		G: uint8(sumG / count),
		B: uint8(sumB / count),
		A: uint8(sumA / count),
	}
}

func meanRGBAAbsDelta(a, b *image.NRGBA) float64 {
	if !a.Rect.Eq(b.Rect) {
		return 255
	}

	var total uint64
	for i := 0; i < len(a.Pix); i++ {
		total += uint64(absInt(int(a.Pix[i]) - int(b.Pix[i])))
	}

	return float64(total) / float64(len(a.Pix))
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
