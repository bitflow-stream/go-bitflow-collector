package psutil

import (
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/bitflow-stream/go-bitflow"
	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

type processCpuCollector struct {
}

func (col *processCpuCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	return collector.MetricReaderMap{
		parent.prefix() + "/cpu": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.cpu.GetDiff()
			}),
	}
}

func (col *processCpuCollector) updateProc(info *processInfo) error {
	if cpu, err := info.Times(); err != nil {
		return fmt.Errorf("Failed to get CPU info: %v", err)
	} else {
		busy := (cpu.Total() - cpu.Idle) * cpu_factor
		info.cpu.Add(collector.StoredValue(busy))
	}
	return nil
}

type processDiskCollector struct {
}

func (col *processDiskCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	prefix := parent.prefix()
	return collector.MetricReaderMap{
		prefix + "/disk/read": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioRead.GetDiff()
			}),
		prefix + "/disk/write": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioWrite.GetDiff()
			}),
		prefix + "/disk/io": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioTotal.GetDiff()
			}),
		prefix + "/disk/readBytes": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioReadBytes.GetDiff()
			}),
		prefix + "/disk/writeBytes": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioWriteBytes.GetDiff()
			}),
		prefix + "/disk/ioBytes": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ioBytesTotal.GetDiff()
			}),
	}
}

func (col *processDiskCollector) updateProc(info *processInfo) error {
	if io, err := info.IOCounters(); err != nil {
		return fmt.Errorf("Failed to get disk-IO info: %v", err)
	} else {
		info.ioRead.Add(collector.StoredValue(io.ReadCount))
		info.ioWrite.Add(collector.StoredValue(io.WriteCount))
		info.ioTotal.Add(collector.StoredValue(io.ReadCount + io.WriteCount))
		info.ioReadBytes.Add(collector.StoredValue(io.ReadBytes))
		info.ioWriteBytes.Add(collector.StoredValue(io.WriteBytes))
		info.ioBytesTotal.Add(collector.StoredValue(io.ReadBytes + io.WriteBytes))
	}
	return nil
}

type processMemoryCollector struct {
}

func (col *processMemoryCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	prefix := parent.prefix()
	return collector.MetricReaderMap{
		prefix + "/mem/rss": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return bitflow.Value(proc.mem_rss)
			}),
		prefix + "/mem/vms": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return bitflow.Value(proc.mem_vms)
			}),
		prefix + "/mem/swap": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return bitflow.Value(proc.mem_swap)
			}),
	}
}

func (col *processMemoryCollector) updateProc(info *processInfo) error {
	// Alternative: col.MemoryInfoEx()
	if mem, err := info.MemoryInfo(); err != nil {
		return fmt.Errorf("Failed to get memory info: %v", err)
	} else {
		info.mem_rss = mem.RSS
		info.mem_vms = mem.VMS
		info.mem_swap = mem.Swap
	}
	return nil
}

type processNetCollector struct {
}

func (col *processNetCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	prefix := parent.prefix()
	return collector.MetricReaderMap{
		prefix + "/net-io/bytes": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.Bytes.GetDiff()
			}),
		prefix + "/net-io/packets": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.Packets.GetDiff()
			}),
		prefix + "/net-io/rx_bytes": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.RxBytes.GetDiff()
			}),
		prefix + "/net-io/rx_packets": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.RxPackets.GetDiff()
			}),
		prefix + "/net-io/tx_bytes": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.TxBytes.GetDiff()
			}),
		prefix + "/net-io/tx_packets": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.TxPackets.GetDiff()
			}),
		prefix + "/net-io/errors": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.Errors.GetDiff()
			}),
		prefix + "/net-io/dropped": parent.netIoSum(
			func(proc *processInfo) bitflow.Value {
				return proc.net.Dropped.GetDiff()
			}),
	}
}

func (col *processNetCollector) updateProc(info *processInfo) error {
	// Alternative: col.Connections()
	if counters, err := info.NetIOCounters(false); err != nil {
		return fmt.Errorf("Failed to get net-IO info: %v", err)
	} else {
		if len(counters) != 1 {
			return fmt.Errorf("gopsutil/process/Process.NetIOCounters() returned %v NetIOCountersStat instead of %v", len(counters), 1)
		}
		info.net.Add(&counters[0])
	}
	return nil
}

type processFdCollector struct {
}

func (col *processFdCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	return collector.MetricReaderMap{
		parent.prefix() + "/fds": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return bitflow.Value(proc.numFds)
			}),
	}
}

func (col *processFdCollector) updateProc(info *processInfo) error {
	// Alternative: col.NumFDs(), proc.OpenFiles()
	if num, err := col.procNumFds(info); err != nil {
		return fmt.Errorf("Failed to get number of open files: %v", err)
	} else {
		info.numFds = num
	}
	return nil
}

func (col *processFdCollector) procNumFds(info *processInfo) (int32, error) {
	// This is part of gopsutil/process.Process.fillFromfd()
	pid := info.Pid
	statPath := hostProcFile(strconv.Itoa(int(pid)), "fd")
	d, err := os.Open(statPath)
	if err != nil {
		return 0, err
	}
	defer d.Close()
	fileNames, err := d.Readdirnames(-1)
	numFDs := int32(len(fileNames))
	return numFDs, err
}

type processMiscCollector struct {
}

func (col *processMiscCollector) metrics(parent *ProcessCollector) collector.MetricReaderMap {
	prefix := parent.prefix()
	return collector.MetricReaderMap{
		prefix + "/threads": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return bitflow.Value(proc.numThreads)
			}),

		prefix + "/ctxSwitch": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ctxSwitchVoluntary.GetDiff()
			}),
		prefix + "/ctxSwitch/voluntary": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ctxSwitchInvoluntary.GetDiff()
			}),
		prefix + "/ctxSwitch/involuntary": parent.sum(
			func(proc *processInfo) bitflow.Value {
				return proc.ctxSwitchInvoluntary.GetDiff() + proc.ctxSwitchVoluntary.GetDiff()
			}),
	}
}

func (col *processMiscCollector) updateProc(info *processInfo) error {
	// Misc, Alternative: col.NumThreads(), col.NumCtxSwitches()
	if numThreads, ctxSwitches, err := col.procGetMisc(info); err != nil {
		return fmt.Errorf("Failed to get number of threads/ctx-switches: %v", err)
	} else {
		info.numThreads = numThreads
		info.ctxSwitchVoluntary.Add(collector.StoredValue(ctxSwitches.Voluntary))
		info.ctxSwitchInvoluntary.Add(collector.StoredValue(ctxSwitches.Involuntary))
	}
	return nil
}

func (col *processMiscCollector) procGetMisc(info *processInfo) (numThreads int32, numCtxSwitches process.NumCtxSwitchesStat, err error) {
	// This is part of gopsutil/process.Process.fillFromStatus()
	pid := info.Pid
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
