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
	pids        map[int32]*SingleProcessCollector
}

func (col *PsutilProcessCollector) Init() error {
	col.own_pid = int32(os.Getpid())
	col.cpu_factor = 100 / float64(runtime.NumCPU())
	col.Reset(col)

	prefix := "proc/" + col.GroupName
	col.Readers = map[string]collector.MetricReader{
		prefix + "/num": col.readNumProc,
		prefix + "/cpu": col.readCpu,

		prefix + "/disk/read":       col.readIoRead,
		prefix + "/disk/write":      col.readIoWrite,
		prefix + "/disk/io":         col.readIo,
		prefix + "/disk/readBytes":  col.readBytesRead,
		prefix + "/disk/writeBytes": col.readBytesWrite,
		prefix + "/disk/ioBytes":    col.readBytes,

		prefix + "/ctxSwitch":             col.readCtxSwitch,
		prefix + "/ctxSwitch/voluntary":   col.readCtxSwitchVoluntary,
		prefix + "/ctxSwitch/involuntary": col.readCtxSwitchInvoluntary,

		prefix + "/mem/rss":  col.readMemRss,
		prefix + "/mem/vms":  col.readMemVms,
		prefix + "/mem/swap": col.readMemSwap,
		prefix + "/fds":      col.readFds,
		prefix + "/threads":  col.readThreads,

		prefix + "/net-io/bytes":      col.readNetBytes,
		prefix + "/net-io/packets":    col.readNetPackets,
		prefix + "/net-io/rx_bytes":   col.readNetRxBytes,
		prefix + "/net-io/rx_packets": col.readNetRxPackets,
		prefix + "/net-io/tx_bytes":   col.readNetTxBytes,
		prefix + "/net-io/tx_packets": col.readNetTxPackets,
		prefix + "/net-io/errors":     col.readNetErrors,
		prefix + "/net-io/dropped":    col.readNetDropped,
	}
	return nil
}

func (col *PsutilProcessCollector) Update() (err error) {
	if err := col.updatePids(); err != nil {
		return err
	}
	col.updateProcesses()
	if err == nil {
		col.UpdateMetrics()
	}
	return
}

func (col *PsutilProcessCollector) updatePids() error {
	if col.pidsUpdated {
		return nil
	}

	newPids := make(map[int32]*SingleProcessCollector)
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
				procCollector, ok := col.pids[pid]
				if !ok {
					procCollector = col.MakeProcessCollector(proc, col.Factory)
				}
				newPids[pid] = procCollector
				break
			}
		}
	}
	if len(newPids) == 0 && errors > 0 && col.PrintErrors {
		log.Errorln("Warning: Observing no processes, failed to check", errors, "out of", len(pids), "PIDs")
	}
	col.pids = newPids

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
	for pid, proc := range col.pids {
		if err := proc.update(); err != nil {
			// Process probably does not exist anymore
			delete(col.pids, pid)
			if col.PrintErrors {
				log.WithField("pid", pid).Warnln("Process info update failed:", err)
			}
		}
	}
}

func (col *PsutilProcessCollector) readNumProc() bitflow.Value {
	return bitflow.Value(len(col.pids))
}

func (col *PsutilProcessCollector) readCpu() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.cpu.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readIoRead() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioRead.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readIoWrite() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioWrite.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readIo() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioTotal.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readBytesRead() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioReadBytes.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readBytesWrite() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioWriteBytes.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readBytes() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ioBytesTotal.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readCtxSwitchVoluntary() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ctxSwitchVoluntary.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readCtxSwitchInvoluntary() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ctxSwitchInvoluntary.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readCtxSwitch() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.ctxSwitchInvoluntary.GetDiff())
		res += bitflow.Value(proc.ctxSwitchVoluntary.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readMemRss() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.mem_rss)
	}
	return
}

func (col *PsutilProcessCollector) readMemVms() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.mem_vms)
	}
	return
}

func (col *PsutilProcessCollector) readMemSwap() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.mem_swap)
	}
	return
}

func (col *PsutilProcessCollector) readFds() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.numFds)
	}
	return
}

func (col *PsutilProcessCollector) readThreads() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.numThreads)
	}
	return
}

func (col *PsutilProcessCollector) readNetBytes() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.Bytes.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetPackets() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.Packets.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetRxBytes() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.RxBytes.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetRxPackets() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.RxPackets.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetTxBytes() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.TxBytes.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetTxPackets() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.TxPackets.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetErrors() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.Errors.GetDiff())
	}
	return
}

func (col *PsutilProcessCollector) readNetDropped() (res bitflow.Value) {
	for _, proc := range col.pids {
		res += bitflow.Value(proc.net.Dropped.GetDiff())
	}
	return
}
