package libvirt

import "github.com/antongulenko/go-bitflow-collector"

type cpuCollector struct {
	vmSubCollectorImpl
	cpu_total   *collector.ValueRing
	cpu_system  *collector.ValueRing
	cpu_user    *collector.ValueRing
	cpu_virtual *collector.ValueRing
}

func NewCpuCollector(parent *vmCollector) *cpuCollector {
	factory := parent.parent.factory
	return &cpuCollector{
		vmSubCollectorImpl: parent.child("cpu"),
		cpu_system:         factory.NewValueRing(),
		cpu_user:           factory.NewValueRing(),
		cpu_total:          factory.NewValueRing(),
		cpu_virtual:        factory.NewValueRing(),
	}
}

func (col *cpuCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "cpu":        col.cpu_total.GetDiff,
		prefix + "cpu/user":   col.cpu_user.GetDiff,
		prefix + "cpu/system": col.cpu_system.GetDiff,
		prefix + "cpu/virt":   col.cpu_virtual.GetDiff,
	}
}

func (col *cpuCollector) Update() error {
	if stats, err := col.parent.domain.CpuStats(); err != nil {
		return err
	} else {
		col.cpu_total.Add(LogbackCpuVal(stats.CpuTime))
		col.cpu_user.Add(LogbackCpuVal(stats.UserTime))
		col.cpu_system.Add(LogbackCpuVal(stats.SystemTime))
		col.cpu_virtual.Add(LogbackCpuVal(stats.VcpuTime))
		return nil
	}
}
