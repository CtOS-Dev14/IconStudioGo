package peicon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"os"
	"sort"

	xdraw "golang.org/x/image/draw"
)

// WriteSmartUHDICO creates a complete UHD multi-size icon pack (.ico)
func WriteSmartUHDICO(layers []IconLayerInfo, rawMap map[int][]byte, outPath string) error {
	if len(layers) == 0 {
		return fmt.Errorf("저장할 아이콘 레이어가 없습니다")
	}

	// Check if we already have a UHD (>= 256px) layer
	has256 := false
	for _, l := range layers {
		if l.Width >= 256 {
			has256 = true
			break
		}
	}

	// 1. If 256px exists, preserve all original layers
	if has256 {
		count := uint16(len(layers))
		header := make([]byte, 6)
		binary.LittleEndian.PutUint16(header[0:2], 0) // Reserved
		binary.LittleEndian.PutUint16(header[2:4], 1) // Type 1 = Icon
		binary.LittleEndian.PutUint16(header[4:6], count)

		offset := uint32(6 + (16 * int(count)))
		var dirBytes bytes.Buffer
		var dataBytes bytes.Buffer

		for _, item := range layers {
			raw := rawMap[item.IconID]
			wByte := byte(item.RawW)
			hByte := byte(item.RawH)
			if item.Width >= 256 {
				wByte = 0
			}
			if item.Height >= 256 {
				hByte = 0
			}

			entryBytes := make([]byte, 16)
			entryBytes[0] = wByte
			entryBytes[1] = hByte
			entryBytes[2] = byte(item.Colors)
			entryBytes[3] = 0 // Reserved
			binary.LittleEndian.PutUint16(entryBytes[4:6], uint16(item.Planes))
			binary.LittleEndian.PutUint16(entryBytes[6:8], uint16(item.BitCount))
			binary.LittleEndian.PutUint32(entryBytes[8:12], uint32(len(raw)))
			binary.LittleEndian.PutUint32(entryBytes[12:16], offset)

			dirBytes.Write(entryBytes)
			dataBytes.Write(raw)
			offset += uint32(len(raw))
		}

		var fileBuf bytes.Buffer
		fileBuf.Write(header)
		fileBuf.Write(dirBytes.Bytes())
		fileBuf.Write(dataBytes.Bytes())

		return os.WriteFile(outPath, fileBuf.Bytes(), 0644)
	}

	// 2. Legacy icon without 256px: Upscale to complete master pack [256, 128, 64, 48, 32, 16]
	sorted := make([]IconLayerInfo, len(layers))
	copy(sorted, layers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Width > sorted[j].Width
	})

	bestEntry := sorted[0]
	raw := rawMap[bestEntry.IconID]
	masterImg, err := DecodeIconLayer(raw, bestEntry)
	if err != nil {
		return fmt.Errorf("원본 레이어 디코딩 실패: %w", err)
	}

	targetSizes := []int{256, 128, 64, 48, 32, 16}
	type encodedChunk struct {
		size int
		data []byte
	}
	chunks := make([]encodedChunk, 0, len(targetSizes))

	for _, sz := range targetSizes {
		var resized image.Image
		bounds := masterImg.Bounds()
		if bounds.Dx() == sz && bounds.Dy() == sz {
			resized = masterImg
		} else {
			dst := image.NewRGBA(image.Rect(0, 0, sz, sz))
			xdraw.CatmullRom.Scale(dst, dst.Bounds(), masterImg, bounds, draw.Over, nil)
			resized = dst
		}

		var pngBuf bytes.Buffer
		if err := png.Encode(&pngBuf, resized); err != nil {
			return fmt.Errorf("PNG 인코딩 실패 (크기 %d): %w", sz, err)
		}
		chunks = append(chunks, encodedChunk{
			size: sz,
			data: pngBuf.Bytes(),
		})
	}

	count := uint16(len(chunks))
	header := make([]byte, 6)
	binary.LittleEndian.PutUint16(header[0:2], 0)
	binary.LittleEndian.PutUint16(header[2:4], 1)
	binary.LittleEndian.PutUint16(header[4:6], count)

	offset := uint32(6 + (16 * int(count)))
	var dirBytes bytes.Buffer
	var bodyBytes bytes.Buffer

	for _, ch := range chunks {
		wByte := byte(ch.size)
		if ch.size >= 256 {
			wByte = 0
		}

		entry := make([]byte, 16)
		entry[0] = wByte
		entry[1] = wByte
		entry[2] = 0 // colors
		entry[3] = 0 // reserved
		binary.LittleEndian.PutUint16(entry[4:6], 1)                  // Planes
		binary.LittleEndian.PutUint16(entry[6:8], 32)                 // 32-bit RGBA
		binary.LittleEndian.PutUint32(entry[8:12], uint32(len(ch.data)))
		binary.LittleEndian.PutUint32(entry[12:16], offset)

		dirBytes.Write(entry)
		bodyBytes.Write(ch.data)
		offset += uint32(len(ch.data))
	}

	var finalFile bytes.Buffer
	finalFile.Write(header)
	finalFile.Write(dirBytes.Bytes())
	finalFile.Write(bodyBytes.Bytes())

	return os.WriteFile(outPath, finalFile.Bytes(), 0644)
}

// WriteUHDPNG extracts the highest quality layer as a crisp PNG
func WriteUHDPNG(layers []IconLayerInfo, rawMap map[int][]byte, outPath string) error {
	if len(layers) == 0 {
		return fmt.Errorf("저장할 아이콘 레이어가 없습니다")
	}

	sorted := make([]IconLayerInfo, len(layers))
	copy(sorted, layers)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Width > sorted[j].Width
	})

	best := sorted[0]
	raw := rawMap[best.IconID]

	// If raw is already a PNG and >= 256px, write raw directly
	if len(raw) >= 8 && bytes.Equal(raw[:8], pngHeader) && best.Width >= 256 {
		return os.WriteFile(outPath, raw, 0644)
	}

	img, err := DecodeIconLayer(raw, best)
	if err != nil {
		return fmt.Errorf("아이콘 디코딩 실패: %w", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() < 256 || bounds.Dy() < 256 {
		dst := image.NewRGBA(image.Rect(0, 0, 256, 256))
		xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)
		img = dst
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return png.Encode(outFile, img)
}
