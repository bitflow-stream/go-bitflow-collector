package psutil

import "github.com/bitflow-stream/go-bitflow-collector"

type RootCollector struct {
	collector.AbstractCollector

	Factory *collector.ValueRingFactory

	pids      *PidCollector
	cpu       *CpuCollector
	mem       *MemCollector
	load      *LoadCollector
	net       *NetCollector
	netProto  *NetProtoCollector
	diskIo    *DiskIOCollector
	diskUsage *DiskUsageCollector
}

func NewPsutilRootCollector(factory *collector.ValueRingFactory) *RootCollector {
	col := &RootCollector{
		AbstractCollector: collector.RootCollector("psutil"),
		Factory:           factory,
	}

	col.pids = newPidCollector(col)
	col.cpu = newCpuCollector(col)
	col.mem = newMemCollector(col)
	col.load = newLoadCollector(col)
	col.net = newNetCollector(col)
	col.netProto = newNetProtoCollector(col)
	col.diskIo = newDiskIoCollector(col)
	col.diskUsage = newDiskUsageCollector(col)
	return col
}

func (col *RootCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		col.pids,
		col.cpu,
		col.mem,
		col.load,
		col.net,
		col.netProto,
		col.diskIo,
		col.diskUsage,
	}, nil
}
