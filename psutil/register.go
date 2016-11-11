package psutil

import (
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/process"
)

// Global information updated regularly
var osInformation struct {
	pids []int32
}

func RegisterPsutilCollectors(osInfoUpdate time.Duration, factory *collector.ValueRingFactory) {
	collector.RegisterCollector(new(PsutilMemCollector))
	collector.RegisterCollector(&PsutilCpuCollector{Factory: factory})
	collector.RegisterCollector(new(PsutilLoadCollector))
	collector.RegisterCollector(&PsutilNetCollector{Factory: factory})
	collector.RegisterCollector(&PsutilNetProtoCollector{Factory: factory})
	collector.RegisterCollector(&PsutilDiskIOCollector{Factory: factory})
	collector.RegisterCollector(new(PsutilDiskUsageCollector))
	collector.RegisterCollector(new(PsutilMiscCollector))
	go UpdateRunningPids(osInfoUpdate)
}

// This is required for PsutilMiscCollector and PsutilProcessCollector
func UpdateRunningPids(interval time.Duration) {
	for {
		if pids, err := process.Pids(); err != nil {
			log.Errorln("Failed to update PIDs:", err)
		} else {
			osInformation.pids = pids
		}
		time.Sleep(interval)
	}
}
