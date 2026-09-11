package peicon

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"

	xdraw "golang.org/x/image/draw"
	xbmp "golang.org/x/image/bmp"
)

var pngHeader = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// DecodeIconLayer decodes raw icon bytes (PNG or DIB) into an image.Image
func DecodeIconLayer(raw []byte, entry IconLayerInfo) (image.Image, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("빈 아이콘 데이터")
	}

	// 1. Check if PNG
	if len(raw) >= 8 && bytes.Equal(raw[:8], pngHeader) {
		img, err := png.Decode(bytes.NewReader(raw))
		if err == nil {
			return img, nil
		}
	}

	// 2. DIB Bitmap decoding
	if len(raw) >= 40 {
		headerSize := binary.LittleEndian.Uint32(raw[0:4])
		if headerSize >= 40 {
			w := int(int32(binary.LittleEndian.Uint32(raw[4:8])))
			h := int(int32(binary.LittleEndian.Uint32(raw[8:12])))
			planes := binary.LittleEndian.Uint16(raw[12:14])
			bpp := binary.LittleEndian.Uint16(raw[14:16])
			comp := binary.LittleEndian.Uint32(raw[16:20])
			imgSz := binary.LittleEndian.Uint32(raw[20:24])
			clrUsed := binary.LittleEndian.Uint32(raw[32:36])

			realH := int(math.Abs(float64(h))) / 2
			if realH == 0 {
				realH = entry.Height
			}
			if w == 0 {
				w = entry.Width
			}

			// If 32-bit uncompressed, decode directly to handle alpha channel accurately
			if bpp == 32 && comp == 0 && len(raw) >= 40+w*realH*4 {
				rgba := image.NewRGBA(image.Rect(0, 0, w, realH))
				pixelOffset := 40
				hasNonZeroAlpha := false

				// Scan pixels bottom-to-top (Windows DIB standard)
				for y := realH - 1; y >= 0; y-- {
					for x := 0; x < w; x++ {
						idx := pixelOffset + (y*w+x)*4
						if idx+3 < len(raw) {
							b := raw[idx]
							g := raw[idx+1]
							r := raw[idx+2]
							a := raw[idx+3]
							if a > 0 {
								hasNonZeroAlpha = true
							}
							rgba.SetRGBA(x, realH-1-y, color.RGBA{R: r, G: g, B: b, A: a})
						}
					}
				}

				// If alpha channel is all zeros, examine AND mask or set alpha to 255
				if !hasNonZeroAlpha {
					andMaskOffset := 40 + (w * realH * 4)
					andRowBytes := ((w + 31) / 32) * 4
					if len(raw) >= andMaskOffset+(andRowBytes*realH) {
						for y := 0; y < realH; y++ {
							maskRow := andMaskOffset + ((realH - 1 - y) * andRowBytes)
							for x := 0; x < w; x++ {
								byteVal := raw[maskRow+(x/8)]
								bit := (byteVal >> (7 - (x % 8))) & 1
								origColor := rgba.RGBAAt(x, y)
								if bit == 1 {
									rgba.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 0})
								} else {
									rgba.SetRGBA(x, y, color.RGBA{R: origColor.R, G: origColor.G, B: origColor.B, A: 255})
								}
							}
						}
					} else {
						// Fallback: all opaque
						for y := 0; y < realH; y++ {
							for x := 0; x < w; x++ {
								c := rgba.RGBAAt(x, y)
								rgba.SetRGBA(x, y, color.RGBA{R: c.R, G: c.G, B: c.B, A: 255})
							}
						}
					}
				}

				return rgba, nil
			}

			// For 1, 4, 8, 16, 24 bpp: construct standard BMP and decode using xbmp
			clrCount := clrUsed
			if clrCount == 0 && bpp <= 8 {
				clrCount = 1 << bpp
			}
			offsetToBits := 14 + 40 + (clrCount * 4)
			bmpFileSize := 14 + uint32(len(raw))

			bmpData := make([]byte, 0, bmpFileSize)
			buf := bytes.NewBuffer(bmpData)

			// 14-byte BMP Header
			buf.Write([]byte{'B', 'M'})
			binary.Write(buf, binary.LittleEndian, bmpFileSize)
			binary.Write(buf, binary.LittleEndian, uint16(0))
			binary.Write(buf, binary.LittleEndian, uint16(0))
			binary.Write(buf, binary.LittleEndian, offsetToBits)

			// 40-byte DIB Info Header with fixed height
			binary.Write(buf, binary.LittleEndian, uint32(40))
			binary.Write(buf, binary.LittleEndian, int32(w))
			binary.Write(buf, binary.LittleEndian, int32(realH))
			binary.Write(buf, binary.LittleEndian, planes)
			binary.Write(buf, binary.LittleEndian, bpp)
			binary.Write(buf, binary.LittleEndian, comp)
			binary.Write(buf, binary.LittleEndian, imgSz)
			binary.Write(buf, binary.LittleEndian, int32(0))
			binary.Write(buf, binary.LittleEndian, int32(0))
			binary.Write(buf, binary.LittleEndian, clrUsed)
			binary.Write(buf, binary.LittleEndian, uint32(0))

			// Remaining data (palette + XOR bits)
			buf.Write(raw[40:])

			img, err := xbmp.Decode(bytes.NewReader(buf.Bytes()))
			if err == nil {
				return img, nil
			}
		}
	}

	// 3. Fallback blank transparent image
	w := entry.Width
	if w <= 0 {
		w = 16
	}
	h := entry.Height
	if h <= 0 {
		h = 16
	}
	return image.NewRGBA(image.Rect(0, 0, w, h)), nil
}

// GenerateThumbnailBase64 creates a high quality thumbnail encoded as a base64 PNG data URL
func GenerateThumbnailBase64(raw []byte, entry IconLayerInfo, thumbSize int) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("빈 데이터")
	}

	// 1. If raw is already a UHD/High-Res PNG, return lossless original data URL directly!
	// This preserves 100% original UHD crisp quality without any recompression artifacts.
	if len(raw) >= 8 && bytes.Equal(raw[:8], pngHeader) && entry.Width >= 128 && entry.Width <= 256 {
		encoded := base64.StdEncoding.EncodeToString(raw)
		return "data:image/png;base64," + encoded, nil
	}

	img, err := DecodeIconLayer(raw, entry)
	if err != nil {
		return "", err
	}

	bounds := img.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	var finalImg image.Image
	if srcW <= thumbSize && srcH <= thumbSize {
		finalImg = img
	} else {
		dst := image.NewRGBA(image.Rect(0, 0, thumbSize, thumbSize))
		// Use draw.Src (NOT draw.Over) to avoid black halo / alpha border artifacts
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
		finalImg = dst
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, finalImg); err != nil {
		return "", err
	}

	encoded := base64.StdEncoding.EncodeToString(buf.Bytes())
	return "data:image/png;base64," + encoded, nil
}
