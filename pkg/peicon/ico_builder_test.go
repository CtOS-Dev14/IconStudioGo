package peicon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractIconAndPNG(t *testing.T) {
	target := `C:\Windows\System32\shell32.dll`
	data, err := OpenAndParsePE(target)
	if err != nil {
		t.Fatalf("OpenAndParsePE failed: %v", err)
	}

	tempDir, err := os.MkdirTemp("", "iconstudio_test_*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Pick first group
	var testGrpID string
	var testLayers []IconLayerInfo
	for gid, layers := range data.GroupIcons {
		testGrpID = gid
		testLayers = layers
		break
	}

	icoOut := filepath.Join(tempDir, "test_icon_"+testGrpID+".ico")
	err = WriteSmartUHDICO(testLayers, data.RawIcons, icoOut)
	if err != nil {
		t.Fatalf("WriteSmartUHDICO failed: %v", err)
	}

	stat, err := os.Stat(icoOut)
	if err != nil || stat.Size() == 0 {
		t.Fatalf("ICO file missing or empty: %v", err)
	}
	t.Logf("Generated ICO size: %d bytes", stat.Size())

	pngOut := filepath.Join(tempDir, "test_icon_"+testGrpID+".png")
	err = WriteUHDPNG(testLayers, data.RawIcons, pngOut)
	if err != nil {
		t.Fatalf("WriteUHDPNG failed: %v", err)
	}

	pngStat, err := os.Stat(pngOut)
	if err != nil || pngStat.Size() == 0 {
		t.Fatalf("PNG file missing or empty: %v", err)
	}
	t.Logf("Generated PNG size: %d bytes", pngStat.Size())
}
