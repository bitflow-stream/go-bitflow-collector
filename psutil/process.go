package psutil

import (
	"os"
	"regexp"
	"runtime"
	"sync"
	"time"

	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/shirou/gopsutil/process"
	log "github.com/sirupsen/logrus"
)

var (
	PidUpdateInterval = 60 * time.Second

	own_pid    = int32(os.Getpid())
	cpu_factor = 100 / float64(runtime.NumCPU())
)

type ProcessCollector struct {
	collector.AbstractCollector
	factory         *collector.ValueRingFactory
	cmdlineFilter   []*regexp.Regexp
	groupName       string
	printErrors     bool
	includeChildren bool
	pids            *PidCollector

	pidsUpdated bool
	procs       map[int32]*processInfo
	procsLock   sync.RWMutex
}

func (col *RootCollector) NewProcessCollector(filter []*regexp.Regexp, name string, printErrors bool, includeChildProcesses bool) *ProcessCollector {
	return &ProcessCollector{
		AbstractCollector: col.Child(name),
		cmdlineFilter:     filter,
		groupName:         name,
		printErrors:       printErrors,
		includeChildren:   includeChildProcesses,
		factory:           col.Factory,
		pids:              col.pids,
	}
}

func (col *RootCollector) NewMultiProcessCollector(name string) *MultiProcessCollector {
	return &MultiProcessCollector{
		AbstractCollector: col.Child(name),
		root:              col,
	}
}

type MultiProcessCollector struct {
	collector.AbstractCollector
	root                *RootCollector
	Processes           []ProcessCollectorDescription
	descriptionsChanged bool
}

type ProcessCollectorDescription struct {
	Name                  string
	Filter                []*regexp.Regexp
	PrintErrors           bool
	IncludeChildProcesses bool
}

func (multi *MultiProcessCollector) UpdateProcesses() {
	multi.descriptionsChanged = true
}

func (multi *MultiProcessCollector) Init() ([]collector.Collector, error) {
	cols := make([]collector.Collector, len(multi.Processes))
	for i, params := range multi.Processes {
		cols[i] = multi.root.NewProcessCollector(params.Filter, params.Name, params.PrintErrors, params.IncludeChildProcesses)
	}
	multi.descriptionsChanged = false
	return cols, nil
}

func (multi *MultiProcessCollector) Depends() []collector.Collector {
	return []collector.Collector{multi.root}
}

func (multi *MultiProcessCollector) Update() error {
	if multi.descriptionsChanged {
		return collector.MetricsChanged
	}
	return nil
}

func (multi *MultiProcessCollector) MetricsChanged() error {
	return multi.Update()
}

func (col *ProcessCollector) Init() ([]collector.Collector, error) {
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

func (col *ProcessCollector) Metrics() collector.MetricReaderMap {
	return collector.MetricReaderMap{
		col.prefix() + "/num": func() bitflow.Value {
			return bitflow.Value(len(col.procs))
		},
	}
}

func (col *ProcessCollector) Depends() []collector.Collector {
	return []collector.Collector{col.pids}
}

func (col *ProcessCollector) Update() error {
	return col.updatePids()
}

func (col *ProcessCollector) updatePids() error {
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
				newProcs[pid] = col.getProcInfo(pid, proc)
				break
			}
		}
	}
	if col.includeChildren {
		pidList := make([]*processInfo, 0, len(newProcs))
		for _, proc := range newProcs {
			pidList = append(pidList, proc)
		}
		for _, proc := range pidList {
			col.addChildren(proc.Process, newProcs)
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

func (col *ProcessCollector) getProcInfo(pid int32, proc *process.Process) *processInfo {
	col.procsLock.RLock()
	procCollector, ok := col.procs[pid]
	col.procsLock.RUnlock()
	if !ok {
		procCollector = col.newProcess(proc)
	}
	return procCollector
}

func (col *ProcessCollector) addChildren(proc *process.Process, newProcs map[int32]*processInfo) {
	children, err := proc.Children()
	if err == process.ErrorNoChildren {
		return
	}
	if err != nil {
		log.WithField("pid", proc.Pid).Warnln("Obtaining child processes of", proc.Pid, "failed:", err)
		return
	}
	for _, child := range children {
		if _, ok := newProcs[child.Pid]; !ok && child.Pid != own_pid {
			newProcs[child.Pid] = col.getProcInfo(child.Pid, child)
		}
		col.addChildren(child, newProcs)
	}
}

func (col *ProcessCollector) newProcess(proc *process.Process) *processInfo {
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

func (col *ProcessCollector) sum(getVal func(*processInfo) bitflow.Value) func() bitflow.Value {
	return func() (res bitflow.Value) {
		col.procsLock.RLock()
		defer col.procsLock.RUnlock()
		for _, proc := range col.procs {
			res += getVal(proc)
		}
		return
	}
}

func (col *ProcessCollector) netIoSum(getVal func(*processInfo) bitflow.Value) func() bitflow.Value {
	return func() (res bitflow.Value) {
		col.procsLock.RLock()
		defer col.procsLock.RUnlock()
		for _, proc := range col.procs {
			res += getVal(proc)

			// TODO HACK
			// Process specific network statistics read from the proc filesystem are not actually network specific,
			// but simply copies of the host-wide statistics. Therefore, do not sum them up, but simply use the info of one process.
			// The data is still not correctly representing the network usage of the processes, but the only way to do that is PCAP
			break
		}
		return
	}
}

func (col *ProcessCollector) prefix() string {
	return "proc/" + col.groupName
}

type processSubCollector struct {
	collector.AbstractCollector
	parent *ProcessCollector
	impl   processSubCollectorImpl
}

type processSubCollectorImpl interface {
	metrics(parent *ProcessCollector) collector.MetricReaderMap
	updateProc(info *processInfo) error
}

func (col *ProcessCollector) Child(name string, impl processSubCollectorImpl) *processSubCollector {
	return &processSubCollector{
		AbstractCollector: col.AbstractCollector.Child(name),
		parent:            col,
		impl:              impl,
	}
}

func (col *processSubCollector) Metrics() collector.MetricReaderMap {
	return col.impl.metrics(col.parent)
}

func (col *processSubCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *processSubCollector) Update() error {
	deletedProcesses := col.doUpdate()
	if len(deletedProcesses) > 0 {
		col.parent.procsLock.Lock()
		defer col.parent.procsLock.Unlock()
		for _, pid := range deletedProcesses {
			delete(col.parent.procs, pid)
		}
	}
	return nil
}

func (col *processSubCollector) doUpdate() (deletedProcesses []int32) {
	col.parent.procsLock.RLock()
	defer col.parent.procsLock.RUnlock()
	for pid, proc := range col.parent.procs {
		if err := col.impl.updateProc(proc); err != nil {
			// Process probably does not exist anymore
			deletedProcesses = append(deletedProcesses, pid)
			if col.parent.printErrors {
				log.WithField("pid", pid).Warnln("Process info update failed:", err)
			}
		}
	}
	return
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
