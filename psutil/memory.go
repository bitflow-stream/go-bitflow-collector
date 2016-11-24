package psutil

import (
	"path/filepath"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/mem"
)

type PsutilMemCollector struct {
	collector.AbstractCollector
	memory *mem.VirtualMemoryStat
}

func newMemCollector(root *PsutilRootCollector) *PsutilMemCollector {
	return &PsutilMemCollector{
		AbstractCollector: root.Child("mem"),
	}
}

func (col *PsutilMemCollector) Update() (err error) {
	col.memory, err = mem.VirtualMemory()
	return
}

func (col *PsutilMemCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		"mem/free":    col.readFreeMem,
		"mem/used":    col.readUsedMem,
		"mem/percent": col.readUsedPercentMem,
	}
}

func (col *PsutilMemCollector) readFreeMem() bitflow.Value {
	return bitflow.Value(col.memory.Available)
}

func (col *PsutilMemCollector) readUsedMem() bitflow.Value {
	return bitflow.Value(col.memory.Used)
}

func (col *PsutilMemCollector) readUsedPercentMem() bitflow.Value {
	return bitflow.Value(col.memory.UsedPercent)
}

func hostProcFile(parts ...string) string {
	// Forbidden import: "github.com/shirou/gopsutil/internal/common"
	// return common.HostProc(parts...)
	all := make([]string, len(parts)+1)
	all[0] = "/proc"
	copy(all[1:], parts)
	return filepath.Join(all...)
}
