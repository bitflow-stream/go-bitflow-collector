package libvirt

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

type vmGeneralReader struct {
	info lib.VirDomainInfo
	cpu  *collector.ValueRing
}

func NewVmGeneralReader(factory *collector.ValueRingFactory) *vmGeneralReader {
	return &vmGeneralReader{
		cpu: factory.NewValueRing(),
	}
}

type LogbackCpuVal uint64

func (val LogbackCpuVal) DiffValue(logback collector.LogbackValue, interval time.Duration) bitflow.Value {
	switch previous := logback.(type) {
	case LogbackCpuVal:
		return bitflow.Value(val-previous) / bitflow.Value(interval.Nanoseconds())
	case *LogbackCpuVal:
		return bitflow.Value(val-*previous) / bitflow.Value(interval.Nanoseconds())
	default:
		log.Errorf("Cannot diff %v (%T) and %v (%T)", val, val, logback, logback)
		return bitflow.Value(0)
	}
}

func (val LogbackCpuVal) AddValue(logback collector.LogbackValue) collector.LogbackValue {
	switch previous := logback.(type) {
	case LogbackCpuVal:
		return collector.StoredValue(val + previous)
	case *LogbackCpuVal:
		return collector.StoredValue(val + *previous)
	default:
		log.Errorf("Cannot add %v (%T) and %v (%T)", val, val, logback, logback)
		return collector.StoredValue(0)
	}
}

func (reader *vmGeneralReader) register(domainName string) map[string]collector.MetricReader {
	return map[string]collector.MetricReader{
		"libvirt/" + domainName + "/general/cpu":    reader.cpu.GetDiff,
		"libvirt/" + domainName + "/general/maxMem": reader.readMaxMem,
		"libvirt/" + domainName + "/general/mem":    reader.readMem,
	}
}

func (reader *vmGeneralReader) description(xmlDesc *xmlpath.Node) {
}

func (reader *vmGeneralReader) update(domain lib.VirDomain) (err error) {
	reader.info, err = domain.GetInfo()
	if err == nil {
		reader.cpu.Add(LogbackCpuVal(reader.info.GetCpuTime()))
	}
	return
}

func (reader *vmGeneralReader) readMaxMem() bitflow.Value {
	return bitflow.Value(reader.info.GetMaxMem())
}

func (reader *vmGeneralReader) readMem() bitflow.Value {
	return bitflow.Value(reader.info.GetMemory())
}
