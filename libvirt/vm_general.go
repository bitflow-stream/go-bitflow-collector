package libvirt

import (
	"time"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	log "github.com/sirupsen/logrus"
)

type vmGeneralCollector struct {
	vmSubCollectorImpl
	info DomainInfo
	cpu  *collector.ValueRing
}

func NewVmGeneralCollector(parent *vmCollector) *vmGeneralCollector {
	return &vmGeneralCollector{
		vmSubCollectorImpl: parent.child("general"),
		cpu:                parent.parent.factory.NewValueRing(),
	}
}

func (col *vmGeneralCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "general/cpu":    col.cpu.GetDiff,
		prefix + "general/maxMem": col.readMaxMem,
		prefix + "general/mem":    col.readMem,
	}
}

func (col *vmGeneralCollector) Update() (err error) {
	col.info, err = col.parent.domain.GetInfo()
	if err == nil {
		col.cpu.Add(LogbackCpuVal(col.info.CpuTime))
	}
	return
}

func (col *vmGeneralCollector) readMaxMem() bitflow.Value {
	return bitflow.Value(col.info.MaxMem)
}

func (col *vmGeneralCollector) readMem() bitflow.Value {
	return bitflow.Value(col.info.Mem)
}

type LogbackCpuVal uint64

func (val LogbackCpuVal) DiffValue(logback collector.LogbackValue, interval time.Duration) bitflow.Value {
	switch previous := logback.(type) {
	case LogbackCpuVal:
		return bitflow.Value(val-previous) / bitflow.Value(interval.Nanoseconds()) * 100
	case *LogbackCpuVal:
		return bitflow.Value(val-*previous) / bitflow.Value(interval.Nanoseconds()) * 100
	default:
		log.Errorf("Cannot diff %v (%T) and %v (%T)", val, val, logback, logback)
		return bitflow.Value(0)
	}
}

func (val LogbackCpuVal) AddValue(logback collector.LogbackValue) collector.LogbackValue {
	switch previous := logback.(type) {
	case LogbackCpuVal:
		return LogbackCpuVal(val + previous)
	case *LogbackCpuVal:
		return LogbackCpuVal(val + *previous)
	default:
		log.Errorf("Cannot add %v (%T) and %v (%T)", val, val, logback, logback)
		return LogbackCpuVal(0)
	}
}

func (val LogbackCpuVal) GetValue() bitflow.Value {
	return bitflow.Value(val)
}
