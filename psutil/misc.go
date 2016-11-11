package psutil

import (
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
)

type PsutilMiscCollector struct {
	collector.AbstractCollector
}

func (col *PsutilMiscCollector) Init() error {
	// TODO missing: number of open files, threads, etc in entire OS
	col.Reset(col)
	col.Readers = map[string]collector.MetricReader{
		"num_procs": col.readNumProcs,
	}
	return nil
}

func (col *PsutilMiscCollector) Update() error {
	col.UpdateMetrics()
	return nil
}

func (col *PsutilMiscCollector) readNumProcs() bitflow.Value {
	return bitflow.Value(len(osInformation.pids))
}
