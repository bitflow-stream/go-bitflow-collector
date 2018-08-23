package libvirt

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

type memoryStatCollector struct {
	vmSubCollectorImpl
	unused    uint64
	available uint64
}

func NewMemoryCollector(parent *vmCollector) *memoryStatCollector {
	return &memoryStatCollector{
		vmSubCollectorImpl: parent.child("mem"),
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
		col.unused = memStats.Unused
		col.available = memStats.Available
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
	avail := col.available
	if avail == 0 {
		return bitflow.Value(0)
	}
	used := avail - col.unused
	return bitflow.Value(used) / bitflow.Value(avail) * 100
}
