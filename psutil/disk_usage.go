package psutil

import (
	"fmt"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/disk"
)

const (
	diskUsagePrefix = "disk-usage/"
	diskUsageAll    = "all"
)

type PsutilDiskUsageCollector struct {
	collector.AbstractCollector
	allPartitions      map[string]string          // partition name -> mountpoint
	observedPartitions map[string]bool            // Set of mountpoints
	usage              map[string]*disk.UsageStat // Keys: mountpoints
}

func (col *PsutilDiskUsageCollector) Init() error {
	col.Reset(col)
	col.usage = make(map[string]*disk.UsageStat)
	col.observedPartitions = make(map[string]bool)

	var err error
	col.allPartitions, err = col.getAllPartitions()
	if err != nil {
		return err
	}

	col.Readers = make(map[string]collector.MetricReader)
	for name, mountpoint := range col.allPartitions {
		name = diskUsagePrefix + name + "/"
		reader := &diskUsageReader{
			col:        col,
			mountpoint: mountpoint,
		}
		col.Readers[name+"free"] = reader.readFree
		col.Readers[name+"used"] = reader.readPercent
	}
	reader := &allDiskUsageReader{col}
	col.Readers[diskUsagePrefix+diskUsageAll+"/free"] = reader.readFree
	col.Readers[diskUsagePrefix+diskUsageAll+"/used"] = reader.readPercent
	return nil
}

func (col *PsutilDiskUsageCollector) Collect(metric *collector.Metric) error {
	lastSlash := strings.LastIndex(metric.name, "/")
	partition := metric.name[len(diskUsagePrefix):lastSlash]
	if partition == diskUsageAll {
		for _, mountpoint := range col.allPartitions {
			col.observedPartitions[mountpoint] = true
		}
	} else {
		mountpoint := col.allPartitions[partition]
		col.observedPartitions[mountpoint] = true
	}
	return col.AbstractCollector.Collect(metric)
}

func (col *PsutilDiskUsageCollector) getAllPartitions() (map[string]string, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	result := make(map[string]string, len(partitions))
	for _, partition := range partitions {
		result[col.partitionName(partition)] = partition.Mountpoint
	}
	return result, nil
}

// should return a system-wide unique name
func (col *PsutilDiskUsageCollector) partitionName(partition disk.PartitionStat) string {
	dev := partition.Device
	lastSpace := strings.LastIndex(dev, "/")
	if lastSpace >= 0 {
		dev = dev[lastSpace+1:]
	}
	return dev
}

func (col *PsutilDiskUsageCollector) update() error {
	for mountpoint := range col.observedPartitions {
		usage, err := disk.Usage(mountpoint)
		if err != nil {
			return fmt.Errorf("Error reading disk-usage of disk mounted at %v: %v", mountpoint, err)
		}
		col.usage[mountpoint] = usage
	}
	return nil
}

func (col *PsutilDiskUsageCollector) checkChangedPartitions() error {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return err
	}
	checked := make(map[string]bool, len(partitions))
	for _, partition := range partitions {
		name := col.partitionName(partition)
		if _, ok := col.allPartitions[name]; !ok {
			return collector.MetricsChanged
		}
		checked[name] = true
	}
	if len(checked) != len(col.allPartitions) {
		return collector.MetricsChanged
	}
	return nil
}

func (col *PsutilDiskUsageCollector) Update() error {
	if err := col.checkChangedPartitions(); err != nil {
		return err
	}
	if err := col.update(); err == nil {
		col.UpdateMetrics()
		return nil
	} else {
		return err
	}
}

func (col *PsutilDiskUsageCollector) getStats(mountpoint string) *disk.UsageStat {
	if disk, ok := col.usage[mountpoint]; ok {
		return disk
	} else {
		log.Warnln("disk-usage counters not found for partition at", mountpoint)
		return nil
	}
}

type diskUsageReader struct {
	col        *PsutilDiskUsageCollector
	mountpoint string
}

func (reader *diskUsageReader) readFree() (res bitflow.Value) {
	if stats := reader.col.getStats(reader.mountpoint); stats != nil {
		res = bitflow.Value(stats.Free)
	}
	return
}

func (reader *diskUsageReader) readPercent() (res bitflow.Value) {
	if stats := reader.col.getStats(reader.mountpoint); stats != nil {
		res = bitflow.Value(stats.UsedPercent)
	}
	return
}

type allDiskUsageReader struct {
	col *PsutilDiskUsageCollector
}

func (reader *allDiskUsageReader) readFree() (res bitflow.Value) {
	for part := range reader.col.observedPartitions {
		if stats := reader.col.getStats(part); stats != nil {
			res += bitflow.Value(stats.Free)
		}
	}
	return
}

func (reader *allDiskUsageReader) readPercent() bitflow.Value {
	var used, total uint64
	for part := range reader.col.observedPartitions {
		if stats := reader.col.getStats(part); stats != nil {
			used += stats.Used
			total += stats.Total
		}
	}
	if total == 0 {
		return bitflow.Value(0)
	} else {
		return bitflow.Value(used) / bitflow.Value(total) * 100
	}
}
