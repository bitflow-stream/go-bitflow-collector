package psutil

import (
	"fmt"
	"time"

	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/shirou/gopsutil/cpu"
	log "github.com/sirupsen/logrus"
)

type CpuCollector struct {
	collector.AbstractCollector
	factory *collector.ValueRingFactory
	ring    *collector.ValueRing
}

func newCpuCollector(root *RootCollector) *CpuCollector {
	return &CpuCollector{
		AbstractCollector: root.Child("cpu"),
		factory:           root.Factory,
	}
}

func (col *CpuCollector) Init() ([]collector.Collector, error) {
	col.ring = col.factory.NewValueRing()
	return nil, nil
}

func (col *CpuCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		"cpu": col.ring.GetDiff,
	}
}

func (col *CpuCollector) Update() (err error) {
	times, err := cpu.Times(false)
	if err == nil {
		if len(times) != 1 {
			err = fmt.Errorf("gopsutil/cpu.Times() returned %v cpu.TimesStat instead of %v", len(times), 1)
		} else {
			col.ring.Add(&cpuTime{times[0]})
		}
	}
	return
}

type cpuTime struct {
	cpu.TimesStat
}

func (t *cpuTime) getAllBusy() (float64, float64) {
	busy := t.User + t.System + t.Nice + t.Irq +
		t.Softirq + t.Steal + t.Guest + t.GuestNice + t.Stolen
	return busy + t.Idle + t.Iowait, busy
}

func (t *cpuTime) DiffValue(logback collector.LogbackValue, _ time.Duration) bitflow.Value {
	if previous, ok := logback.(*cpuTime); ok {
		// Calculation based on https://github.com/shirou/gopsutil/blob/master/cpu/cpu_unix.go
		t1All, t1Busy := previous.getAllBusy()
		t2All, t2Busy := t.getAllBusy()

		if t2Busy <= t1Busy {
			return 0
		}
		if t2All <= t1All {
			return 1
		}
		return bitflow.Value((t2Busy - t1Busy) / (t2All - t1All) * 100)
	} else {
		log.Errorf("Cannot diff %v (%T) and %v (%T)", t, t, logback, logback)
		return bitflow.Value(0)
	}
}

func (t *cpuTime) AddValue(incoming collector.LogbackValue) collector.LogbackValue {
	if other, ok := incoming.(*cpuTime); ok {
		return &cpuTime{
			cpu.TimesStat{
				User:      t.User + other.User,
				System:    t.System + other.System,
				Idle:      t.Idle + other.Idle,
				Nice:      t.Nice + other.Nice,
				Iowait:    t.Iowait + other.Iowait,
				Irq:       t.Irq + other.Irq,
				Softirq:   t.Softirq + other.Softirq,
				Steal:     t.Steal + other.Steal,
				Guest:     t.Guest + other.Guest,
				GuestNice: t.GuestNice + other.GuestNice,
				Stolen:    t.Stolen + other.Stolen,
			},
		}
	} else {
		log.Errorf("Cannot add %v (%T) and %v (%T)", t, t, incoming, incoming)
		return collector.StoredValue(0)
	}
}

func (t *cpuTime) GetValue() bitflow.Value {
	_, busy := t.getAllBusy()
	return bitflow.Value(busy)
}
