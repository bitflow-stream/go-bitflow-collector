package psutil

import (
	"fmt"

	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/shirou/gopsutil/process"
)

type PidCollector struct {
	collector.AbstractCollector
	pids []int32
}

func newPidCollector(root *RootCollector) *PidCollector {
	return &PidCollector{
		AbstractCollector: root.Child("pids"),
	}
}

func (col *PidCollector) Metrics() collector.MetricReaderMap {
	// TODO missing: number of open files, threads, etc in entire OS
	return collector.MetricReaderMap{
		"num_procs": col.readNumProcs,
	}
}

func (col *PidCollector) Update() (err error) {
	if col.pids, err = process.Pids(); err != nil {
		err = fmt.Errorf("Failed to update PIDs: %v", err)
	}
	return
}

func (col *PidCollector) readNumProcs() bitflow.Value {
	return bitflow.Value(len(col.pids))
}
