package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"IconStudio/pkg/peicon"
	"IconStudio/pkg/settings"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"strings"
)

const (
	AppName     = "Icon Studio"
	AppVersion  = "v2.2.4"
	AppBuild    = "2.2.4.0"
	AppDesc     = "Windows UHD Multi-size Master Pack Icon Studio (Go Engine 2.2)"
	CompanyName = "CTOS"
	ProductName = "Icon Studio Pro UHD 2.2"
)

// App struct
type App struct {
	ctx             context.Context
	mu              sync.RWMutex
	currentFilePath string
	parsedData      *peicon.ParsedIconData
	currentSettings settings.AppSettings
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		currentSettings: settings.LoadSettings(),
	}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Register drag and drop callback
	runtime.OnFileDrop(ctx, func(x, y int, paths []string) {
		if len(paths) > 0 {
			runtime.EventsEmit(ctx, "file-dropped", paths[0])
		}
	})
}

// onSecondInstanceLaunch is called when a second instance is launched
func (a *App) onSecondInstanceLaunch(secondInstanceData options.SecondInstanceData) {
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
	// Bring to foreground in Windows
	runtime.WindowSetAlwaysOnTop(a.ctx, true)
	runtime.WindowSetAlwaysOnTop(a.ctx, false)

	// If any supported file was passed as argument, open it immediately
	for _, arg := range secondInstanceData.Args {
		if strings.HasPrefix(arg, "-") {
			continue
		}
		ext := strings.ToLower(filepath.Ext(arg))
		if ext == ".exe" || ext == ".dll" || ext == ".mun" || ext == ".ocx" || ext == ".cpl" || ext == ".scr" || ext == ".ico" {
			runtime.EventsEmit(a.ctx, "file-dropped", arg)
			break
		}
	}
}

// domReady is called when the frontend DOM is ready
func (a *App) domReady(ctx context.Context) {
	s := a.currentSettings

	// Restore window geometry
	if s.HasPosition && s.X >= -100 && s.Y >= -100 {
		runtime.WindowSetPosition(ctx, s.X, s.Y)
	} else {
		runtime.WindowCenter(ctx)
	}

	if s.Width >= 840 && s.Height >= 540 {
		runtime.WindowSetSize(ctx, s.Width, s.Height)
	}

	if s.IsMaximized {
		runtime.WindowMaximise(ctx)
	}

	a.applyNativeTheme(s.Theme)
}

// beforeClose is called before the window closes
func (a *App) beforeClose(ctx context.Context) bool {
	a.CaptureWindowGeometry()
	settings.SaveSettings(a.currentSettings)
	return false // Allow closing
}

// CaptureWindowGeometry records the current window position and size
func (a *App) CaptureWindowGeometry() {
	if a.ctx == nil {
		return
	}
	isMax := runtime.WindowIsMaximised(a.ctx)
	a.currentSettings.IsMaximized = isMax

	if !isMax {
		w, h := runtime.WindowGetSize(a.ctx)
		x, y := runtime.WindowGetPosition(a.ctx)
		if w >= 840 && h >= 540 {
			a.currentSettings.Width = w
			a.currentSettings.Height = h
		}
		if x >= -200 && y >= -200 {
			a.currentSettings.X = x
			a.currentSettings.Y = y
			a.currentSettings.HasPosition = true
		}
	}
}

func (a *App) applyNativeTheme(theme string) {
	if a.ctx == nil {
		return
	}
	switch theme {
	case "dark":
		runtime.WindowSetDarkTheme(a.ctx)
	case "light":
		runtime.WindowSetLightTheme(a.ctx)
	default:
		runtime.WindowSetSystemDefaultTheme(a.ctx)
	}
}

// GetSavedSettings returns the user settings loaded from disk
func (a *App) GetSavedSettings() settings.AppSettings {
	return a.currentSettings
}

// SaveTheme updates and persists the chosen theme
func (a *App) SaveTheme(theme string) {
	a.currentSettings.Theme = theme
	a.applyNativeTheme(theme)
	settings.SaveSettings(a.currentSettings)
}

// SaveWindowGeometry called from frontend on resize/move debounce
func (a *App) SaveWindowGeometry() {
	a.CaptureWindowGeometry()
	settings.SaveSettings(a.currentSettings)
}

// AppInfo returns application metadata
func (a *App) GetAppInfo() map[string]string {
	return map[string]string{
		"name":        AppName,
		"version":     AppVersion,
		"build":       AppBuild,
		"description": AppDesc,
		"company":     CompanyName,
		"product":     ProductName,
	}
}

// OpenFile opens a native Windows file dialog and parses the selected binary
func (a *App) OpenFile() (*peicon.ParseResult, error) {
	selected, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "아이콘을 추출할 바이너리 파일 선택",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "바이너리 및 DLL (*.exe;*.dll;*.mun;*.scr;*.ocx;*.cpl)",
				Pattern:     "*.exe;*.dll;*.mun;*.scr;*.ocx;*.cpl",
			},
			{
				DisplayName: "모든 파일 (*.*)",
				Pattern:     "*.*",
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("파일 선택 실패: %w", err)
	}
	if selected == "" {
		return nil, nil // User cancelled
	}

	return a.LoadFile(selected)
}

// LoadFile parses a specified PE/MUN file path
func (a *App) LoadFile(filePath string) (*peicon.ParseResult, error) {
	data, err := peicon.OpenAndParsePE(filePath)
	if err != nil {
		return nil, err
	}

	a.mu.Lock()
	a.parsedData = data
	a.currentFilePath = filePath
	a.mu.Unlock()

	// Build summary list with parallel thumbnail generation using Goroutines
	groupCount := len(data.GroupIcons)
	summaries := make([]peicon.IconGroupSummary, groupCount)

	type task struct {
		idx    int
		grpID  string
		layers []peicon.IconLayerInfo
	}

	tasks := make(chan task, groupCount)
	var wg sync.WaitGroup
	numWorkers := 8

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for t := range tasks {
				hasUHD := false
				maxRes := 0
				maxBpp := 0

				for _, l := range t.layers {
					if l.Width >= 256 {
						hasUHD = true
					}
					if l.Width > maxRes {
						maxRes = l.Width
					}
					if l.BitCount > maxBpp {
						maxBpp = l.BitCount
					}
				}

				// Find highest resolution and quality layer for crisp large thumbnail
				var thumbEntry peicon.IconLayerInfo
				for _, l := range t.layers {
					if thumbEntry.IconID == 0 {
						thumbEntry = l
						continue
					}
					// Prefer larger resolution, then higher bit count
					if l.Width > thumbEntry.Width || (l.Width == thumbEntry.Width && l.BitCount > thumbEntry.BitCount) {
						thumbEntry = l
					}
				}

				var thumbBase64 string
				raw := data.RawIcons[thumbEntry.IconID]
				if len(raw) > 0 {
					// Generate 128px ultra crisp thumbnail
					thumbBase64, _ = peicon.GenerateThumbnailBase64(raw, thumbEntry, 128)
				}

				summaries[t.idx] = peicon.IconGroupSummary{
					GroupID:     t.grpID,
					TotalLayers: len(t.layers),
					HasUHD:      hasUHD,
					MaxRes:      maxRes,
					MaxBpp:      maxBpp,
					Thumbnail:   thumbBase64,
					Layers:      t.layers,
				}
			}
		}()
	}

	idx := 0
	for grpID, layers := range data.GroupIcons {
		tasks <- task{idx: idx, grpID: grpID, layers: layers}
		idx++
	}
	close(tasks)
	wg.Wait()

	return &peicon.ParseResult{
		FilePath:    filePath,
		FileName:    filepath.Base(filePath),
		TotalGroups: len(summaries),
		Groups:      summaries,
	}, nil
}

// ExtractSingle saves a single icon group to ICO or PNG
func (a *App) ExtractSingle(groupID string, format string) (string, error) {
	a.mu.RLock()
	data := a.parsedData
	a.mu.RUnlock()

	if data == nil {
		return "", fmt.Errorf("분석된 파일이 없습니다")
	}

	layers, ok := data.GroupIcons[groupID]
	if !ok || len(layers) == 0 {
		return "", fmt.Errorf("그룹 #%s 아이콘을 찾을 수 없습니다", groupID)
	}

	ext := ".ico"
	filterName := "UHD Icon (*.ico)"
	filterPattern := "*.ico"
	if format == "png" {
		ext = ".png"
		filterName = "PNG Image (*.png)"
		filterPattern = "*.png"
	}

	savePath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "아이콘 저장",
		DefaultFilename: fmt.Sprintf("icon_%s%s", groupID, ext),
		Filters: []runtime.FileFilter{
			{DisplayName: filterName, Pattern: filterPattern},
		},
	})
	if err != nil {
		return "", fmt.Errorf("저장 대화상자 오류: %w", err)
	}
	if savePath == "" {
		return "", nil // Cancelled
	}

	if format == "ico" {
		err = peicon.WriteSmartUHDICO(layers, data.RawIcons, savePath)
	} else {
		err = peicon.WriteUHDPNG(layers, data.RawIcons, savePath)
	}

	if err != nil {
		return "", fmt.Errorf("파일 저장 실패: %w", err)
	}

	return savePath, nil
}

// BatchExtract saves multiple icon groups to a chosen folder
func (a *App) BatchExtract(groupIDs []string, format string) (int, error) {
	a.mu.RLock()
	data := a.parsedData
	a.mu.RUnlock()

	if data == nil {
		return 0, fmt.Errorf("분석된 파일이 없습니다")
	}
	if len(groupIDs) == 0 {
		return 0, fmt.Errorf("선택된 아이콘이 없습니다")
	}

	targetDir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "저장할 대상 폴더 선택",
	})
	if err != nil {
		return 0, fmt.Errorf("폴더 선택 오류: %w", err)
	}
	if targetDir == "" {
		return 0, nil // Cancelled
	}

	successCount := 0
	for _, gid := range groupIDs {
		layers, ok := data.GroupIcons[gid]
		if !ok || len(layers) == 0 {
			continue
		}

		outPath := filepath.Join(targetDir, fmt.Sprintf("icon_%s.%s", gid, format))
		var errSave error
		if format == "ico" {
			errSave = peicon.WriteSmartUHDICO(layers, data.RawIcons, outPath)
		} else {
			errSave = peicon.WriteUHDPNG(layers, data.RawIcons, outPath)
		}

		if errSave == nil {
			successCount++
		}
	}

	return successCount, nil
}

// OpenInExplorer opens the folder containing a saved file in Windows Explorer
func (a *App) OpenInExplorer(filePath string) error {
	dir := filepath.Dir(filePath)
	if fi, err := os.Stat(filePath); err == nil && fi.IsDir() {
		dir = filePath
	}
	runtime.BrowserOpenURL(a.ctx, dir)
	return nil
}
