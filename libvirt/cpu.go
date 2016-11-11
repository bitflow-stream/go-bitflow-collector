package libvirt

import (
	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

const (
	MAX_NUM_CPU_STATS               = 4
	VIR_DOMAIN_CPU_STATS_CPUTIME    = "cpu_time" // Total CPU (VM + hypervisor)
	VIR_DOMAIN_CPU_STATS_SYSTEMTIME = "system_time"
	VIR_DOMAIN_CPU_STATS_USERTIME   = "user_time"
	VIR_DOMAIN_CPU_STATS_VCPUTIME   = "vcpu_time" // Excluding hypervisor usage
)

type cpuStatReader struct {
	cpu_total  *collector.ValueRing
	cpu_system *collector.ValueRing
	cpu_user   *collector.ValueRing
	cpu_virt   *collector.ValueRing
}

func NewCpuStatReader(factory *collector.ValueRingFactory) *cpuStatReader {
	return &cpuStatReader{
		cpu_system: factory.NewValueRing(),
		cpu_user:   factory.NewValueRing(),
		cpu_total:  factory.NewValueRing(),
		cpu_virt:   factory.NewValueRing(),
	}
}

func (reader *cpuStatReader) register(domainName string) map[string]collector.MetricReader {
	return map[string]collector.MetricReader{
		"libvirt/" + domainName + "/cpu":        reader.cpu_total.GetDiff,
		"libvirt/" + domainName + "/cpu/user":   reader.cpu_user.GetDiff,
		"libvirt/" + domainName + "/cpu/system": reader.cpu_system.GetDiff,
		"libvirt/" + domainName + "/cpu/virt":   reader.cpu_virt.GetDiff,
	}
}

func (reader *cpuStatReader) description(xmlDesc *xmlpath.Node) {
}

func (reader *cpuStatReader) update(domain lib.VirDomain) error {
	stats := make(lib.VirTypedParameters, MAX_NUM_CPU_STATS)
	// Less detailed alternative: domain.GetVcpus()
	if _, err := domain.GetCPUStats(&stats, len(stats), -1, 1, NO_FLAGS); err != nil {
		return err
	} else {
		for _, param := range stats {
			val, ok := param.Value.(uint64)
			if !ok {
				continue
			}
			switch param.Name {
			case VIR_DOMAIN_CPU_STATS_CPUTIME:
				reader.cpu_total.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_USERTIME:
				reader.cpu_user.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_SYSTEMTIME:
				reader.cpu_system.Add(LogbackCpuVal(val))
			case VIR_DOMAIN_CPU_STATS_VCPUTIME:
				reader.cpu_virt.Add(LogbackCpuVal(val))
			}
		}
		return nil
	}
}
