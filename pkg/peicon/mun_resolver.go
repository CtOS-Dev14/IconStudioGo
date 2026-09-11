package peicon

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modKernel32                        = syscall.NewLazyDLL("kernel32.dll")
	procWow64DisableWow64FsRedirection = modKernel32.NewProc("Wow64DisableWow64FsRedirection")
	procWow64RevertWow64FsRedirection  = modKernel32.NewProc("Wow64RevertWow64FsRedirection")
)

// WithFsRedirectionDisabled runs the provided function with 64-bit file system redirection disabled
func WithFsRedirectionDisabled(fn func() error) error {
	var oldVal uintptr
	if procWow64DisableWow64FsRedirection.Find() == nil {
		r1, _, _ := procWow64DisableWow64FsRedirection.Call(uintptr(unsafe.Pointer(&oldVal)))
		if r1 != 0 {
			defer procWow64RevertWow64FsRedirection.Call(oldVal)
		}
	}
	return fn()
}

// ResolveRealResourceFile checks if a corresponding .mun file exists in SystemResources
func ResolveRealResourceFile(fpath string) string {
	cleanPath := filepath.Clean(fpath)
	fileName := filepath.Base(cleanPath)
	lowerName := strings.ToLower(fileName)

	if strings.HasSuffix(lowerName, ".mun") {
		return cleanPath
	}

	sysRoot := os.Getenv("SystemRoot")
	if sysRoot == "" {
		sysRoot = "C:\\Windows"
	}

	// 1. Check SystemResources directly
	munCandidate := filepath.Join(sysRoot, "SystemResources", fileName+".mun")
	if _, err := os.Stat(munCandidate); err == nil {
		return munCandidate
	}

	// 2. If path is inside system32 or syswow64, check SystemResources
	lowerPath := strings.ToLower(cleanPath)
	if strings.Contains(lowerPath, "system32") || strings.Contains(lowerPath, "syswow64") {
		if _, err := os.Stat(munCandidate); err == nil {
			return munCandidate
		}
	}

	return cleanPath
}
