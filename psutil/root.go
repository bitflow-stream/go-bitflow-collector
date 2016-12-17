package psutil

import "github.com/antongulenko/go-bitflow-collector"

type PsutilRootCollector struct {
	collector.AbstractCollector

	Factory *collector.ValueRingFactory

	pids      *PsutilPidCollector
	cpu       *PsutilCpuCollector
	mem       *PsutilMemCollector
	load      *PsutilLoadCollector
	net       *PsutilNetCollector
	netProto  *PsutilNetProtoCollector
	diskIo    *PsutilDiskIOCollector
	diskUsage *PsutilDiskUsageCollector
}

func NewPsutilRootCollector(factory *collector.ValueRingFactory) *PsutilRootCollector {
	col := &PsutilRootCollector{
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

func (col *PsutilRootCollector) Init() ([]collector.Collector, error) {
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
