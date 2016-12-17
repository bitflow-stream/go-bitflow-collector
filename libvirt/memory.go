package libvirt

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

const (
	//VIR_DOMAIN_MEMORY_STAT_SWAP_IN  = 0
	//VIR_DOMAIN_MEMORY_STAT_SWAP_OUT = 1
	//VIR_DOMAIN_MEMORY_STAT_MAJOR_FAULT = 2
	//VIR_DOMAIN_MEMORY_STAT_MINOR_FAULT = 3
	//VIR_DOMAIN_MEMORY_STAT_ACTUAL_BALLOON = 6
	//VIR_DOMAIN_MEMORY_STAT_RSS            = 7

	VIR_DOMAIN_MEMORY_STAT_UNUSED    = 4
	VIR_DOMAIN_MEMORY_STAT_AVAILABLE = 5
)

type memoryStatCollector struct {
	vmSubcollectorImpl
	unused    uint64
	available uint64
}

func NewMemoryCollector(parent *vmCollector) *memoryStatCollector {
	return &memoryStatCollector{
		vmSubcollectorImpl: parent.child("mem"),
	}
}

func (col *memoryStatCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "mem/available": col.readAvailable,
		prefix + "mem/used":      col.readUsed,
		prefix + "mem/percent":   col.readPercent,
	}
}

func (col *memoryStatCollector) Update() error {
	if memStats, err := col.parent.domain.MemoryStats(); err != nil {
		return err
	} else {
		foundAvailable := false
		foundUnused := false
		var available, unused uint64
		for _, stat := range memStats {
			switch stat.Tag {
			case VIR_DOMAIN_MEMORY_STAT_AVAILABLE:
				available = stat.Val
				foundAvailable = true
			case VIR_DOMAIN_MEMORY_STAT_UNUSED:
				unused = stat.Val
				foundUnused = true
			}
		}
		if !foundAvailable || !foundUnused {
			unused = 0
			available = 0
		}
		col.unused = unused
		col.available = available
		return nil
	}
}

func (col *memoryStatCollector) readAvailable() bitflow.Value {
	return bitflow.Value(col.available)
}

func (col *memoryStatCollector) readUsed() bitflow.Value {
	return bitflow.Value(col.available - col.unused)
}

func (col *memoryStatCollector) readPercent() bitflow.Value {
	used := col.available - col.unused
	return bitflow.Value(used) / bitflow.Value(col.available)
}
