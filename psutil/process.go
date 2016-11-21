package psutil

import (
	"flag"
	"os"
	"regexp"
	"runtime"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

// TODO HACK: automatically find out if this should be excluded
var nopcap = false

func init() {
	flag.BoolVar(&nopcap, "nopcap", nopcap, "Disable PCAP package capturing.")
}

type PsutilProcessCollector struct {
	collector.AbstractCollector
	Factory *collector.ValueRingFactory

	// Settings
	CmdlineFilter     []*regexp.Regexp
	GroupName         string
	PrintErrors       bool
	PidUpdateInterval time.Duration

	pidsUpdated bool
	own_pid     int32
	cpu_factor  float64
	procs       map[int32]*SingleProcessCollector
	net_pcap    BaseNetIoCounters
}

func (col *PsutilProcessCollector) Init() error {
	col.own_pid = int32(os.Getpid())
	col.net_pcap = NewBaseNetIoCounters(col.Factory)
	col.cpu_factor = 100 / float64(runtime.NumCPU())
	col.Reset(col)

	prefix := "proc/" + col.GroupName
	col.Readers = map[string]collector.MetricReader{
		prefix + "/num": func() bitflow.Value {
			return bitflow.Value(len(col.procs))
		},
		prefix + "/cpu": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.cpu.GetDiff()
			}),

		prefix + "/disk/read": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioRead.GetDiff()
			}),
		prefix + "/disk/write": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioWrite.GetDiff()
			}),
		prefix + "/disk/io": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioTotal.GetDiff()
			}),
		prefix + "/disk/readBytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioReadBytes.GetDiff()
			}),
		prefix + "/disk/writeBytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioWriteBytes.GetDiff()
			}),
		prefix + "/disk/ioBytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ioBytesTotal.GetDiff()
			}),

		prefix + "/ctxSwitch": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ctxSwitchVoluntary.GetDiff()
			}),
		prefix + "/ctxSwitch/voluntary": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ctxSwitchInvoluntary.GetDiff()
			}),
		prefix + "/ctxSwitch/involuntary": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.ctxSwitchInvoluntary.GetDiff() + proc.ctxSwitchVoluntary.GetDiff()
			}),

		prefix + "/mem/rss": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return bitflow.Value(proc.mem_rss)
			}),
		prefix + "/mem/vms": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return bitflow.Value(proc.mem_vms)
			}),
		prefix + "/mem/swap": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return bitflow.Value(proc.mem_swap)
			}),
		prefix + "/fds": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return bitflow.Value(proc.numFds)
			}),
		prefix + "/threads": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return bitflow.Value(proc.numThreads)
			}),

		prefix + "/net-io/bytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.Bytes.GetDiff()
			}),
		prefix + "/net-io/packets": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.Packets.GetDiff()
			}),
		prefix + "/net-io/rx_bytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.RxBytes.GetDiff()
			}),
		prefix + "/net-io/rx_packets": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.RxPackets.GetDiff()
			}),
		prefix + "/net-io/tx_bytes": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.TxBytes.GetDiff()
			}),
		prefix + "/net-io/tx_packets": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.TxPackets.GetDiff()
			}),
		prefix + "/net-io/errors": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.Errors.GetDiff()
			}),
		prefix + "/net-io/dropped": col.sum(
			func(proc *SingleProcessCollector) bitflow.Value {
				return proc.net.Dropped.GetDiff()
			}),

		prefix + "/net-pcap/bytes":      col.net_pcap.Bytes.GetDiff,
		prefix + "/net-pcap/packets":    col.net_pcap.Packets.GetDiff,
		prefix + "/net-pcap/rx_bytes":   col.net_pcap.RxBytes.GetDiff,
		prefix + "/net-pcap/rx_packets": col.net_pcap.RxPackets.GetDiff,
		prefix + "/net-pcap/tx_bytes":   col.net_pcap.TxBytes.GetDiff,
		prefix + "/net-pcap/tx_packets": col.net_pcap.TxPackets.GetDiff,
	}
	return nil
}

func (col *PsutilProcessCollector) Update() (err error) {
	if err := col.updatePids(); err != nil {
		return err
	}
	col.updateProcesses()

	if !nopcap {
		err = col.updatePcapNet()
	}

	if err == nil {
		col.UpdateMetrics()
	}
	return
}

func (col *PsutilProcessCollector) updatePids() error {
	if col.pidsUpdated {
		return nil
	}

	newProcs := make(map[int32]*SingleProcessCollector)
	errors := 0
	pids := osInformation.pids
	for _, pid := range pids {
		if pid == col.own_pid {
			continue
		}
		proc, err := process.NewProcess(pid)
		if err != nil {
			// Process does not exist anymore
			errors++
			if col.PrintErrors {
				log.WithField("pid", pid).Warnln("Checking process failed:", err)
			}
			continue
		}
		cmdline, err := proc.Cmdline()
		if err != nil {
			// Probably a permission error
			errors++
			if col.PrintErrors {
				log.WithField("pid", pid).Warnln("Obtaining process cmdline failed:", err)
			}
			continue
		}
		for _, regex := range col.CmdlineFilter {
			if regex.MatchString(cmdline) {
				procCollector, ok := col.procs[pid]
				if !ok {
					procCollector = col.MakeProcessCollector(proc, col.Factory)
				}
				newProcs[pid] = procCollector
				break
			}
		}
	}
	if len(newProcs) == 0 && errors > 0 && col.PrintErrors {
		log.Errorln("Warning: Observing no processes, failed to check", errors, "out of", len(pids), "PIDs")
	}
	col.procs = newProcs

	if col.PidUpdateInterval > 0 {
		col.pidsUpdated = true
		time.AfterFunc(col.PidUpdateInterval, func() {
			col.pidsUpdated = false
		})
	} else {
		col.pidsUpdated = false
	}
	return nil
}

func (col *PsutilProcessCollector) updateProcesses() {
	for pid, proc := range col.procs {
		if err := proc.update(); err != nil {
			// Process probably does not exist anymore
			delete(col.procs, pid)
			if col.PrintErrors {
				log.WithField("pid", pid).Warnln("Process info update failed:", err)
			}
		}
	}
}

func (col *PsutilProcessCollector) sum(getval func(*SingleProcessCollector) bitflow.Value) func() bitflow.Value {
	return func() (res bitflow.Value) {
		for _, proc := range col.procs {
			res += getval(proc)
		}
		return
	}
}
