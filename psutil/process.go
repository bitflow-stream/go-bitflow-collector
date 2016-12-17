package psutil

import (
	"os"
	"regexp"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

var (
	PidUpdateInterval = 60 * time.Second

	own_pid    = int32(os.Getpid())
	cpu_factor = 100 / float64(runtime.NumCPU())
)

type PsutilProcessCollector struct {
	collector.AbstractCollector
	factory       *collector.ValueRingFactory
	cmdlineFilter []*regexp.Regexp
	groupName     string
	printErrors   bool
	pids          *PsutilPidCollector

	pidsUpdated bool
	procs       map[int32]*processInfo
}

func (root *PsutilRootCollector) NewProcessCollector(filter []*regexp.Regexp, name string, printErrors bool) *PsutilProcessCollector {
	return &PsutilProcessCollector{
		AbstractCollector: root.Child(name),
		cmdlineFilter:     filter,
		groupName:         name,
		printErrors:       printErrors,
		factory:           root.Factory,
		pids:              root.pids,
	}
}

func (col *PsutilProcessCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		col.Child("cpu", new(processCpuCollector)),
		col.Child("disk", new(processDiskCollector)),
		col.Child("mem", new(processMemoryCollector)),
		col.Child("net", new(processNetCollector)),
		col.newProcessPcapCollector(),
		col.Child("fd", new(processFdCollector)),
		col.Child("misc", new(processMiscCollector)),
	}, nil
}

func (col *PsutilProcessCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		col.prefix() + "/num": func() bitflow.Value {
			return bitflow.Value(len(col.procs))
		},
	}
}

func (col *PsutilProcessCollector) Depends() []collector.Collector {
	return []collector.Collector{col.pids}
}

func (col *PsutilProcessCollector) Update() error {
	return col.updatePids()
}

func (col *PsutilProcessCollector) updatePids() error {
	if col.pidsUpdated {
		return nil
	}

	newProcs := make(map[int32]*processInfo)
	errors := 0
	pids := col.pids.pids
	for _, pid := range pids {
		if pid == own_pid {
			continue
		}
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Process does not exist anymore
			errors++
			if col.printErrors {
				log.WithField("pid", pid).Warnln("Checking process failed:", err)
			}
			continue
		}
		cmdline, err := proc.Cmdline()
		if err != nil {
			// Probably a permission error
			errors++
			if col.printErrors {
				log.WithField("pid", pid).Warnln("Obtaining process cmdline failed:", err)
			}
			continue
		}
		for _, regex := range col.cmdlineFilter {
			if regex.MatchString(cmdline) {
				procCollector, ok := col.procs[pid]
				if !ok {
					procCollector = col.newProcess(proc)
				}
				newProcs[pid] = procCollector
				break
			}
		}
	}
	if len(newProcs) == 0 && errors > 0 && col.printErrors {
		log.Errorln("Warning: Observing no processes, failed to check", errors, "out of", len(pids), "PIDs")
	}
	col.procs = newProcs

	if PidUpdateInterval > 0 {
		col.pidsUpdated = true
		time.AfterFunc(PidUpdateInterval, func() {
			col.pidsUpdated = false
		})
	} else {
		col.pidsUpdated = false
	}
	return nil
}

func (col *PsutilProcessCollector) newProcess(proc *process.Process) *processInfo {
	return &processInfo{
		Process:              proc,
		cpu:                  col.factory.NewValueRing(),
		ioRead:               col.factory.NewValueRing(),
		ioWrite:              col.factory.NewValueRing(),
		ioTotal:              col.factory.NewValueRing(),
		ioReadBytes:          col.factory.NewValueRing(),
		ioWriteBytes:         col.factory.NewValueRing(),
		ioBytesTotal:         col.factory.NewValueRing(),
		ctxSwitchVoluntary:   col.factory.NewValueRing(),
		ctxSwitchInvoluntary: col.factory.NewValueRing(),
		net:                  NewNetIoCounters(col.factory),
		net_pcap:             NewBaseNetIoCounters(col.factory),
	}
}

func (col *PsutilProcessCollector) sum(getval func(*processInfo) bitflow.Value) func() bitflow.Value {
	return func() (res bitflow.Value) {
		for _, proc := range col.procs {
			res += getval(proc)
		}
		return
	}
}

func (col *PsutilProcessCollector) prefix() string {
	return "proc/" + col.groupName
}

type processSubcollector struct {
	collector.AbstractCollector
	parent *PsutilProcessCollector
	impl   processSubcollectorImpl
}

type processSubcollectorImpl interface {
	metrics(parent *PsutilProcessCollector) collector.MetricReaderMap
	updateProc(info *processInfo) error
}

func (col *PsutilProcessCollector) Child(name string, impl processSubcollectorImpl) *processSubcollector {
	return &processSubcollector{
		AbstractCollector: col.AbstractCollector.Child(name),
		parent:            col,
		impl:              impl,
	}
}

func (col *processSubcollector) Metrics() collector.MetricReaderMap {
	return col.impl.metrics(col.parent)
}

func (col *processSubcollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *processSubcollector) Update() error {
	for pid, proc := range col.parent.procs {
		if err := col.impl.updateProc(proc); err != nil {
			// Process probably does not exist anymore
			delete(col.parent.procs, pid)
			if col.parent.printErrors {
				log.WithField("pid", pid).Warnln("Process info update failed:", err)
			}
		}
	}
	return nil
}

type processInfo struct {
	*process.Process

	cpu                  *collector.ValueRing
	ioRead               *collector.ValueRing
	ioWrite              *collector.ValueRing
	ioTotal              *collector.ValueRing
	ioReadBytes          *collector.ValueRing
	ioWriteBytes         *collector.ValueRing
	ioBytesTotal         *collector.ValueRing
	ctxSwitchVoluntary   *collector.ValueRing
	ctxSwitchInvoluntary *collector.ValueRing
	net                  NetIoCounters
	net_pcap             BaseNetIoCounters
	mem_rss              uint64
	mem_vms              uint64
	mem_swap             uint64
	numFds               int32
	numThreads           int32
}
