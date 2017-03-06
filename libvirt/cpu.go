package libvirt

import "github.com/antongulenko/go-bitflow-collector"

const (
	VIR_DOMAIN_CPU_STATS_CPUTIME    = "cpu_time" // Total CPU (VM + hypervisor)
	VIR_DOMAIN_CPU_STATS_SYSTEMTIME = "system_time"
	VIR_DOMAIN_CPU_STATS_USERTIME   = "user_time"
	VIR_DOMAIN_CPU_STATS_VCPUTIME   = "vcpu_time" // Excluding hypervisor usage
)

type cpuCollector struct {
	vmSubcollectorImpl
	cpu_total  *collector.ValueRing
	cpu_system *collector.ValueRing
	cpu_user   *collector.ValueRing
	cpu_virt   *collector.ValueRing
}

func NewCpuCollector(parent *vmCollector) *cpuCollector {
	factory := parent.parent.factory
	return &cpuCollector{
		vmSubcollectorImpl: parent.child("cpu"),
		cpu_system:         factory.NewValueRing(),
		cpu_user:           factory.NewValueRing(),
		cpu_total:          factory.NewValueRing(),
		cpu_virt:           factory.NewValueRing(),
	}
}

func (col *cpuCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "cpu":        col.cpu_total.GetDiff,
		prefix + "cpu/user":   col.cpu_user.GetDiff,
		prefix + "cpu/system": col.cpu_system.GetDiff,
		prefix + "cpu/virt":   col.cpu_virt.GetDiff,
	}
}

func (col *cpuCollector) Update() error {
	if stats, err := col.parent.domain.CpuStats(); err != nil {
		return err
	} else {
		for name, val := range stats {
			val, ok := val.(uint64)
			if !ok {
				continue
			}
			switch name {
			case VIR_DOMAIN_CPU_STATS_CPUTIME:
				col.cpu_total.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_USERTIME:
				col.cpu_user.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_SYSTEMTIME:
				col.cpu_system.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_VCPUTIME:
				col.cpu_virt.Add(LogbackCpuVal(val))
			}
		}
		return nil
	}
}
