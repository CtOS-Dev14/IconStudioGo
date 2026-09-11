package peicon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"unicode/utf16"
)

const (
	RT_ICON       = 3
	RT_GROUP_ICON = 14
)

type sectionHeader struct {
	VirtualSize   uint32
	VirtualAddr   uint32
	RawDataSize   uint32
	RawDataOffset uint32
}

// PEFile represents a parsed PE file focusing on its resource directory
type PEFile struct {
	data          []byte
	sections      []sectionHeader
	resSectionRVA uint32
	resOffset     uint32
	resSize       uint32
}

// ParsedIconData contains the extracted raw bytes and group metadata
type ParsedIconData struct {
	RawIcons   map[int][]byte            // IconID -> raw bytes
	GroupIcons map[string][]IconLayerInfo // GroupID -> layers
}

// OpenAndParsePE opens a file (handling MUN and redirection) and parses its icons
func OpenAndParsePE(filePath string) (*ParsedIconData, error) {
	realPath := ResolveRealResourceFile(filePath)

	var fileBytes []byte
	err := WithFsRedirectionDisabled(func() error {
		var errRead error
		fileBytes, errRead = os.ReadFile(realPath)
		return errRead
	})
	if err != nil {
		return nil, fmt.Errorf("파일 읽기 실패 (%s): %w", realPath, err)
	}

	return ParsePEBytes(fileBytes)
}

// ParsePEBytes parses PE binary bytes and extracts RT_GROUP_ICON and RT_ICON resources
func ParsePEBytes(data []byte) (*ParsedIconData, error) {
	if len(data) < 64 {
		return nil, errors.New("유효하지 않은 PE 파일: 데이터가 너무 작습니다")
	}

	// 1. DOS Header Check ('MZ')
	if data[0] != 'M' || data[1] != 'Z' {
		return nil, errors.New("MZ 시그니처가 없습니다 (PE 파일 아님)")
	}

	peOffset := binary.LittleEndian.Uint32(data[0x3C:0x40])
	if int(peOffset)+24 > len(data) {
		return nil, errors.New("PE 헤더 오프셋이 범위를 벗어났습니다")
	}

	// 2. PE Signature Check ('PE\0\0')
	if !bytes.Equal(data[peOffset:peOffset+4], []byte{'P', 'E', 0, 0}) {
		return nil, errors.New("PE 시그니처가 일치하지 않습니다")
	}

	coffOffset := peOffset + 4
	numSections := binary.LittleEndian.Uint16(data[coffOffset+2 : coffOffset+4])
	optHeaderSize := binary.LittleEndian.Uint16(data[coffOffset+16 : coffOffset+18])

	optOffset := coffOffset + 20
	if int(optOffset+uint32(optHeaderSize)) > len(data) {
		return nil, errors.New("옵셔널 헤더가 범위를 벗어났습니다")
	}

	optMagic := binary.LittleEndian.Uint16(data[optOffset : optOffset+2])
	var ddOffset uint32
	if optMagic == 0x10b {
		// PE32
		ddOffset = 96
	} else if optMagic == 0x20b {
		// PE32+ (64-bit)
		ddOffset = 112
	} else {
		return nil, fmt.Errorf("알 수 없는 옵셔널 헤더 매직: 0x%x", optMagic)
	}

	// Resource Directory is index 2 in Data Directory
	resEntryOffset := optOffset + ddOffset + (2 * 8)
	if int(resEntryOffset+8) > len(data) {
		return nil, errors.New("리소스 데이터 디렉터리 항목이 없습니다")
	}

	resRVA := binary.LittleEndian.Uint32(data[resEntryOffset : resEntryOffset+4])
	resSize := binary.LittleEndian.Uint32(data[resEntryOffset+4 : resEntryOffset+8])

	if resRVA == 0 || resSize == 0 {
		return nil, errors.New("아이콘 리소스 디렉터리가 비어 있습니다")
	}

	// 3. Parse Section Headers
	secOffset := optOffset + uint32(optHeaderSize)
	sections := make([]sectionHeader, numSections)
	for i := 0; i < int(numSections); i++ {
		curSec := secOffset + uint32(i*40)
		if int(curSec+40) > len(data) {
			break
		}
		sections[i] = sectionHeader{
			VirtualSize:   binary.LittleEndian.Uint32(data[curSec+8 : curSec+12]),
			VirtualAddr:   binary.LittleEndian.Uint32(data[curSec+12 : curSec+16]),
			RawDataSize:   binary.LittleEndian.Uint32(data[curSec+16 : curSec+20]),
			RawDataOffset: binary.LittleEndian.Uint32(data[curSec+20 : curSec+24]),
		}
	}

	pe := &PEFile{
		data:          data,
		sections:      sections,
		resSectionRVA: resRVA,
		resSize:       resSize,
	}

	var ok bool
	pe.resOffset, ok = pe.rvaToOffset(resRVA)
	if !ok || int(pe.resOffset+resSize) > len(data) {
		// 일부 파일의 경우 raw size보다 resSize가 클 수 있으므로 resOffset 유효성만 확인
		if !ok || int(pe.resOffset) >= len(data) {
			return nil, errors.New("리소스 RVA를 파일 오프셋으로 매핑할 수 없습니다")
		}
	}

	return pe.extractIconResources()
}

func (p *PEFile) rvaToOffset(rva uint32) (uint32, bool) {
	for _, sec := range p.sections {
		sz := sec.VirtualSize
		if sec.RawDataSize > sz {
			sz = sec.RawDataSize
		}
		if rva >= sec.VirtualAddr && rva < sec.VirtualAddr+sz {
			diff := rva - sec.VirtualAddr
			return sec.RawDataOffset + diff, true
		}
	}
	return 0, false
}

// readResourceString reads a UTF-16 resource name at a relative offset
func (p *PEFile) readResourceString(relOffset uint32) string {
	absOffset := p.resOffset + relOffset
	if int(absOffset+2) > len(p.data) {
		return ""
	}
	length := binary.LittleEndian.Uint16(p.data[absOffset : absOffset+2])
	absOffset += 2
	if int(absOffset+uint32(length*2)) > len(p.data) {
		return ""
	}

	u16 := make([]uint16, length)
	for i := 0; i < int(length); i++ {
		u16[i] = binary.LittleEndian.Uint16(p.data[absOffset+uint32(i*2) : absOffset+uint32(i*2)+2])
	}
	return string(utf16.Decode(u16))
}

type resEntry struct {
	isNamed  bool
	name     string
	id       uint32
	isSubDir bool
	offset   uint32 // relative to resOffset
}

func (p *PEFile) readDirEntries(dirRelOffset uint32) []resEntry {
	absOffset := p.resOffset + dirRelOffset
	if int(absOffset+16) > len(p.data) {
		return nil
	}

	namedCount := binary.LittleEndian.Uint16(p.data[absOffset+12 : absOffset+14])
	idCount := binary.LittleEndian.Uint16(p.data[absOffset+14 : absOffset+16])
	total := int(namedCount + idCount)

	entries := make([]resEntry, 0, total)
	entryOffset := absOffset + 16

	for i := 0; i < total; i++ {
		if int(entryOffset+8) > len(p.data) {
			break
		}

		nameOrId := binary.LittleEndian.Uint32(p.data[entryOffset : entryOffset+4])
		offsetVal := binary.LittleEndian.Uint32(p.data[entryOffset+4 : entryOffset+8])
		entryOffset += 8

		isSub := (offsetVal & 0x80000000) != 0
		subOffset := offsetVal & 0x7FFFFFFF

		e := resEntry{
			isSubDir: isSub,
			offset:   subOffset,
		}

		if (nameOrId & 0x80000000) != 0 {
			e.isNamed = true
			nameRelOffset := nameOrId & 0x7FFFFFFF
			e.name = p.readResourceString(nameRelOffset)
		} else {
			e.id = nameOrId
		}

		entries = append(entries, e)
	}

	return entries
}

func (p *PEFile) getDataEntryBytes(dataEntryRelOffset uint32) []byte {
	absOffset := p.resOffset + dataEntryRelOffset
	if int(absOffset+16) > len(p.data) {
		return nil
	}

	dataRVA := binary.LittleEndian.Uint32(p.data[absOffset : absOffset+4])
	size := binary.LittleEndian.Uint32(p.data[absOffset+4 : absOffset+8])

	fileOffset, ok := p.rvaToOffset(dataRVA)
	if !ok || int(fileOffset+size) > len(p.data) {
		return nil
	}

	return p.data[fileOffset : fileOffset+size]
}

// resolveDataEntry traverses to a leaf data entry (Language level)
func (p *PEFile) resolveDataEntry(entry resEntry) []byte {
	if !entry.isSubDir {
		return p.getDataEntryBytes(entry.offset)
	}
	// Level 3 (Language level)
	subEntries := p.readDirEntries(entry.offset)
	if len(subEntries) == 0 {
		return nil
	}
	leaf := subEntries[0]
	if leaf.isSubDir {
		return nil
	}
	return p.getDataEntryBytes(leaf.offset)
}

func (p *PEFile) extractIconResources() (*ParsedIconData, error) {
	// Level 1: Root directory
	rootEntries := p.readDirEntries(0)

	var groupIconDirOffset uint32
	var iconDirOffset uint32
	var hasGroup, hasIcon bool

	for _, e := range rootEntries {
		if !e.isNamed && e.isSubDir {
			if e.id == RT_GROUP_ICON {
				groupIconDirOffset = e.offset
				hasGroup = true
			} else if e.id == RT_ICON {
				iconDirOffset = e.offset
				hasIcon = true
			}
		}
	}

	if !hasGroup || !hasIcon {
		return nil, errors.New("아이콘 그룹 리소스(RT_GROUP_ICON 또는 RT_ICON)를 찾을 수 없습니다")
	}

	// 1. Extract RT_ICONs (IconID -> Raw Bytes)
	rawMap := make(map[int][]byte)
	iconEntries := p.readDirEntries(iconDirOffset)
	for _, e := range iconEntries {
		if e.isNamed {
			continue
		}
		iconID := int(e.id)
		dataBytes := p.resolveDataEntry(e)
		if len(dataBytes) > 0 {
			rawMap[iconID] = dataBytes
		}
	}

	// 2. Extract RT_GROUP_ICONs
	groups := make(map[string][]IconLayerInfo)
	groupEntries := p.readDirEntries(groupIconDirOffset)
	for _, ge := range groupEntries {
		var grpIDStr string
		if ge.isNamed {
			grpIDStr = ge.name
		} else {
			grpIDStr = fmt.Sprintf("%d", ge.id)
		}

		gdata := p.resolveDataEntry(ge)
		if len(gdata) < 6 {
			continue
		}

		resCount := binary.LittleEndian.Uint16(gdata[4:6])
		offset := 6
		layers := make([]IconLayerInfo, 0, resCount)

		for i := 0; i < int(resCount); i++ {
			if offset+14 > len(gdata) {
				break
			}
			rawW := int(gdata[offset])
			rawH := int(gdata[offset+1])
			colors := int(gdata[offset+2])
			// offset+3 is reserved
			planes := int(binary.LittleEndian.Uint16(gdata[offset+4 : offset+6]))
			bitCount := int(binary.LittleEndian.Uint16(gdata[offset+6 : offset+8]))
			bytesInRes := int(binary.LittleEndian.Uint32(gdata[offset+8 : offset+12]))
			iconID := int(binary.LittleEndian.Uint16(gdata[offset+12 : offset+14]))
			offset += 14

			width := rawW
			if width == 0 {
				width = 256
			}
			height := rawH
			if height == 0 {
				height = 256
			}

			// Check if the actual raw data is a PNG
			isPNG := false
			if rawBytes, exists := rawMap[iconID]; exists && len(rawBytes) >= 8 {
				if bytes.Equal(rawBytes[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
					isPNG = true
				}
			}

			layers = append(layers, IconLayerInfo{
				Width:    width,
				Height:   height,
				RawW:     rawW,
				RawH:     rawH,
				Colors:   colors,
				Planes:   planes,
				BitCount: bitCount,
				Size:     bytesInRes,
				IconID:   iconID,
				IsPNG:    isPNG,
			})
		}

		if len(layers) > 0 {
			groups[grpIDStr] = layers
		}
	}

	return &ParsedIconData{
		RawIcons:   rawMap,
		GroupIcons: groups,
	}, nil
}
