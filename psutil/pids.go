package psutil

import (
	"fmt"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

type PsutilPidCollector struct {
	collector.AbstractCollector
	pids []int32
}

func newPidCollector(root *PsutilRootCollector) *PsutilPidCollector {
	return &PsutilPidCollector{
		AbstractCollector: root.Child("pids"),
	}
}

func (col *PsutilPidCollector) Metrics() collector.MetricReaderMap {
	// TODO missing: number of open files, threads, etc in entire OS
	return collector.MetricReaderMap{
		"num_procs": col.readNumProcs,
	}
}

func (col *PsutilPidCollector) Update() (err error) {
	if col.pids, err = process.Pids(); err != nil {
		err = fmt.Errorf("Failed to update PIDs: %v", err)
	}
	return
}

func (col *PsutilPidCollector) readNumProcs() bitflow.Value {
	return bitflow.Value(len(col.pids))
}
