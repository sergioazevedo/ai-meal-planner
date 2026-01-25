package metrics

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// SysHealth represents real-time system metrics.
type SysHealth struct {
	AllocMB      uint64
	TotalAllocMB uint64
	SysMB        uint64
	NumGC        uint32
	Goroutines   int
	DataDiskSize string
}

// GetSysHealth collects real-time health data.
func GetSysHealth(dataPath string) SysHealth {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SysHealth{
		AllocMB:      m.Alloc / 1024 / 1024,
		TotalAllocMB: m.TotalAlloc / 1024 / 1024,
		SysMB:        m.Sys / 1024 / 1024,
		NumGC:        m.NumGC,
		Goroutines:   runtime.NumGoroutine(),
		DataDiskSize: calculateDirSize(dataPath),
	}
}

func calculateDirSize(path string) string {
	var size int64
	_ = filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})

	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
