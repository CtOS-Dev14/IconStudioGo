package peicon

import (
	"os"
	"testing"
)

func TestParseWindowsSystemIcon(t *testing.T) {
	targets := []string{
		`C:\Windows\System32\notepad.exe`,
		`C:\Windows\System32\shell32.dll`,
	}

	for _, p := range targets {
		if _, err := os.Stat(p); err != nil {
			continue
		}

		data, err := OpenAndParsePE(p)
		if err != nil {
			t.Fatalf("OpenAndParsePE failed for %s: %v", p, err)
		}

		t.Logf("[%s] found %d group icons", p, len(data.GroupIcons))

		// Check at least one icon
		for grpID, layers := range data.GroupIcons {
			bestEntry := layers[0]
			raw := data.RawIcons[bestEntry.IconID]
			thumb, err := GenerateThumbnailBase64(raw, bestEntry, 64)
			if err != nil {
				t.Fatalf("[%s] Group %s thumbnail failed: %v", p, grpID, err)
			}
			if len(thumb) < 30 {
				t.Fatalf("Thumbnail too short")
			}
			break
		}
	}
}
