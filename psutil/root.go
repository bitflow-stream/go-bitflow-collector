package psutil

import "github.com/antongulenko/go-bitflow-collector"

type PsutilRootCollector struct {
	collector.AbstractCollector

	pids *PsutilPidCollector
	cpu  *PsutilCpuCollector
	//	mem       *PsutilMemCollector
	//	load      *PsutilLoadCollector
	//	net       *PsutilNetCollector
	//	netProto  *PsutilNetProtoCollector
	//	diskIo    *PsutilDiskIOCollector
	diskUsage *PsutilDiskUsageCollector
}

func NewPsutilRootCollector(factory *collector.ValueRingFactory) *PsutilRootCollector {
	col := new(PsutilRootCollector)
	col.Name = "psutil"

	col.pids = newPidCollector(col)
	col.cpu = newCpuCollector(col, factory)
	//	col.mem = newMemCollector(col)
	//	col.load = newLoadCollector(col)
	//	col.net = newNetCollector(col, factory)
	//	col.netProto = newNetProtoCollector(col, factory)
	//	col.diskIo = newDiskIoCollector(col, factory)
	col.diskUsage = newDiskUsageCollector(col)
	return col
}

func (col *PsutilRootCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		col.pids,
		col.cpu,
		//		col.mem,
		//		col.load,
		//		col.net,
		//		col.netProto,
		//		col.diskIo,
		col.diskUsage,
	}, nil
}
