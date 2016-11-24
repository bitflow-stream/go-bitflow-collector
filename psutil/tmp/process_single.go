package psutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

type SingleProcessCollector struct {
	parent *PsutilProcessCollector
	*process.Process
	tasks collector.CollectorTasks

	updateLock           sync.Mutex
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
	mem_rss              uint64
	mem_vms              uint64
	mem_swap             uint64
	numFds               int32
	numThreads           int32
}

func (self *PsutilProcessCollector) MakeProcessCollector(proc *process.Process, factory *collector.ValueRingFactory) *SingleProcessCollector {
	col := &SingleProcessCollector{
		parent:  self,
		Process: proc,

		cpu:                  factory.NewValueRing(),
		ioRead:               factory.NewValueRing(),
		ioWrite:              factory.NewValueRing(),
		ioTotal:              factory.NewValueRing(),
		ioReadBytes:          factory.NewValueRing(),
		ioWriteBytes:         factory.NewValueRing(),
		ioBytesTotal:         factory.NewValueRing(),
		ctxSwitchVoluntary:   factory.NewValueRing(),
		ctxSwitchInvoluntary: factory.NewValueRing(),
		net:                  NewNetIoCounters(factory),
	}
	col.tasks = collector.CollectorTasks{
		col.updateCpu,
		col.updateDisk,
		col.updateMemory,
		col.updateNet,
		col.updateOpenFiles,
		col.updateMisc,
	}
	return col
}

func (col *SingleProcessCollector) update() error {
	return col.tasks.Run()
}

func (col *SingleProcessCollector) updateCpu() error {
	if cpu, err := col.Times(); err != nil {
		return fmt.Errorf("Failed to get CPU info: %v", err)
	} else {
		busy := (cpu.Total() - cpu.Idle) * col.parent.cpu_factor
		col.cpu.Add(collector.StoredValue(busy))
	}
	return nil
}

func (col *SingleProcessCollector) updateDisk() error {
	if io, err := col.IOCounters(); err != nil {
		return fmt.Errorf("Failed to get disk-IO info: %v", err)
	} else {
		col.ioRead.Add(collector.StoredValue(io.ReadCount))
		col.ioWrite.Add(collector.StoredValue(io.WriteCount))
		col.ioTotal.Add(collector.StoredValue(io.ReadCount + io.WriteCount))
		col.ioReadBytes.Add(collector.StoredValue(io.ReadBytes))
		col.ioWriteBytes.Add(collector.StoredValue(io.WriteBytes))
		col.ioBytesTotal.Add(collector.StoredValue(io.ReadBytes + io.WriteBytes))
	}
	return nil
}

func (col *SingleProcessCollector) updateMemory() error {
	// Alternative: col.MemoryInfoEx()
	if mem, err := col.MemoryInfo(); err != nil {
		return fmt.Errorf("Failed to get memory info: %v", err)
	} else {
		col.mem_rss = mem.RSS
		col.mem_vms = mem.VMS
		col.mem_swap = mem.Swap
	}
	return nil
}

func (col *SingleProcessCollector) updateNet() error {
	// Alternative: col.Connections()
	if counters, err := col.NetIOCounters(false); err != nil {
		return fmt.Errorf("Failed to get net-IO info: %v", err)
	} else {
		if len(counters) != 1 {
			return fmt.Errorf("gopsutil/process/Process.NetIOCounters() returned %v NetIOCountersStat instead of %v", len(counters), 1)
		}
		col.net.Add(&counters[0])
	}
	return nil
}

func (col *SingleProcessCollector) updateOpenFiles() error {
	// Alternative: col.NumFDs(), proc.OpenFiles()
	if num, err := col.procNumFds(); err != nil {
		return fmt.Errorf("Failed to get number of open files: %v", err)
	} else {
		col.numFds = num
	}
	return nil
}

func (col *SingleProcessCollector) updateMisc() error {
	// Misc, Alternatice: col.NumThreads(), col.NumCtxSwitches()
	if numThreads, ctxSwitches, err := col.procGetMisc(); err != nil {
		return fmt.Errorf("Failed to get number of threads/ctx-switches: %v", err)
	} else {
		col.numThreads = numThreads
		col.ctxSwitchVoluntary.Add(collector.StoredValue(ctxSwitches.Voluntary))
		col.ctxSwitchInvoluntary.Add(collector.StoredValue(ctxSwitches.Involuntary))
	}
	return nil
}

func (col *SingleProcessCollector) procNumFds() (int32, error) {
	// This is part of gopsutil/process.Process.fillFromfd()
	pid := col.Pid
	statPath := hostProcFile(strconv.Itoa(int(pid)), "fd")
	d, err := os.Open(statPath)
	if err != nil {
		return 0, err
	}
	defer d.Close()
	fnames, err := d.Readdirnames(-1)
	numFDs := int32(len(fnames))
	return numFDs, err
}

func (col *SingleProcessCollector) procGetMisc() (numThreads int32, numCtxSwitches process.NumCtxSwitchesStat, err error) {
	// This is part of gopsutil/process.Process.fillFromStatus()
	pid := col.Pid
	statPath := hostProcFile(strconv.Itoa(int(pid)), "status")
	var contents []byte
	contents, err = ioutil.ReadFile(statPath)
	if err != nil {
		return
	}
	lines := strings.Split(string(contents), "\n")
	leftover_fields := 3
	for _, line := range lines {
		tabParts := strings.SplitN(line, "\t", 2)
		if len(tabParts) < 2 {
			continue
		}
		value := tabParts[1]
		var v int64
		switch strings.TrimRight(tabParts[0], ":") {
		case "Threads":
			v, err = strconv.ParseInt(value, 10, 32)
			if err != nil {
				return
			}
			numThreads = int32(v)
			leftover_fields--
		case "voluntary_ctxt_switches":
			v, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return
			}
			numCtxSwitches.Voluntary = v
			leftover_fields--
		case "nonvoluntary_ctxt_switches":
			v, err = strconv.ParseInt(value, 10, 64)
			if err != nil {
				return
			}
			numCtxSwitches.Involuntary = v
			leftover_fields--
		}
		if leftover_fields <= 0 {
			return
		}
	}
	return
}
