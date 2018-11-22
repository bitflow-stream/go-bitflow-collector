package psutil

import (
	"path/filepath"

	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/shirou/gopsutil/mem"
)

type MemCollector struct {
	collector.AbstractCollector
	memory mem.VirtualMemoryStat
}

func newMemCollector(root *RootCollector) *MemCollector {
	return &MemCollector{
		AbstractCollector: root.Child("mem"),
	}
}

func (col *MemCollector) Update() error {
	memory, err := mem.VirtualMemory()
	if err != nil || memory == nil {
		col.memory = mem.VirtualMemoryStat{}
	} else {
		col.memory = *memory
	}
	return err
}

func (col *MemCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		"mem/free":    col.readFreeMem,
		"mem/used":    col.readUsedMem,
		"mem/percent": col.readUsedPercentMem,
	}
}

func (col *MemCollector) readFreeMem() bitflow.Value {
	return bitflow.Value(col.memory.Available)
}

func (col *MemCollector) readUsedMem() bitflow.Value {
	return bitflow.Value(col.memory.Used)
}

func (col *MemCollector) readUsedPercentMem() bitflow.Value {
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
