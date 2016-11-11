package libvirt

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
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
	MAX_NUM_MEMORY_STATS             = 8
)

type memoryStatReader struct {
	unused    uint64
	available uint64
}

func (reader *memoryStatReader) register(domainName string) map[string]collector.MetricReader {
	return map[string]collector.MetricReader{
		"libvirt/" + domainName + "/mem/available": reader.readAvailable,
		"libvirt/" + domainName + "/mem/used":      reader.readUsed,
		"libvirt/" + domainName + "/mem/percent":   reader.readPercent,
	}
}

func (reader *memoryStatReader) description(xmlDesc *xmlpath.Node) {
}

func (reader *memoryStatReader) update(domain lib.VirDomain) error {
	if memStats, err := domain.MemoryStats(MAX_NUM_MEMORY_STATS, NO_FLAGS); err != nil {
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
		reader.unused = unused
		reader.available = available
		return nil
	}
}

func (reader *memoryStatReader) readAvailable() bitflow.Value {
	return bitflow.Value(reader.available)
}

func (reader *memoryStatReader) readUsed() bitflow.Value {
	return bitflow.Value(reader.available - reader.unused)
}

func (reader *memoryStatReader) readPercent() bitflow.Value {
	used := reader.available - reader.unused
	return bitflow.Value(used) / bitflow.Value(reader.available)
}
