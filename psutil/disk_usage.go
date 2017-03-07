package psutil

import (
	"fmt"
	"strings"

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
	partitions map[string]*diskUsageCollector
}

func newDiskUsageCollector(root *PsutilRootCollector) *PsutilDiskUsageCollector {
	return &PsutilDiskUsageCollector{
		AbstractCollector: root.Child("disk-usage"),
	}
}

func (col *PsutilDiskUsageCollector) Init() ([]collector.Collector, error) {
	col.partitions = make(map[string]*diskUsageCollector)

	partitions, err := col.getAllPartitions()
	if err != nil {
		return nil, err
	}
	result := make([]collector.Collector, 0, len(partitions)+1)
	for name, mountPoint := range partitions {
		diskCollector := &diskUsageCollector{
			AbstractCollector: col.Child(name),
			mountPoint:        mountPoint,
			parent:            col,
		}
		col.partitions[name] = diskCollector
		result = append(result, diskCollector)
	}
	result = append(result, &allDiskUsageCollector{
		AbstractCollector: col.Child(diskUsageAll),
		parent:            col,
	})
	return result, nil
}

func (col *PsutilDiskUsageCollector) Update() error {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return err
	}
	checked := make(map[string]bool, len(partitions))
	for _, partition := range partitions {
		name := col.partitionName(partition)
		if _, ok := col.partitions[name]; !ok {
			return collector.MetricsChanged
		}
		checked[name] = true
	}
	if len(checked) != len(col.partitions) {
		return collector.MetricsChanged
	}
	return nil
}

func (col *PsutilDiskUsageCollector) MetricsChanged() error {
	return col.Update()
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

type diskUsageCollector struct {
	collector.AbstractCollector
	parent     *PsutilDiskUsageCollector
	mountPoint string
	stats      disk.UsageStat
}

func (col *diskUsageCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *diskUsageCollector) Update() error {
	stats, err := disk.Usage(col.mountPoint)
	if err != nil || stats == nil {
		col.stats = disk.UsageStat{}
		err = fmt.Errorf("Error reading disk-usage of disk mounted at %v: %v", col.mountPoint, err)
	} else {
		col.stats = *stats
	}
	return err
}

func (col *diskUsageCollector) Metrics() collector.MetricReaderMap {
	name := diskUsagePrefix + col.Name + "/"
	return collector.MetricReaderMap{
		name + "free": col.readFree,
		name + "used": col.readPercent,
	}
}

func (col *diskUsageCollector) readFree() bitflow.Value {
	return bitflow.Value(col.stats.Free)
}

func (col *diskUsageCollector) readPercent() bitflow.Value {
	return bitflow.Value(col.stats.UsedPercent)
}

type allDiskUsageCollector struct {
	collector.AbstractCollector
	parent *PsutilDiskUsageCollector
}

func (col *allDiskUsageCollector) Depends() []collector.Collector {
	res := make([]collector.Collector, 0, len(col.parent.partitions)+1)
	for _, partition := range col.parent.partitions {
		res = append(res, partition)
	}
	res = append(res, col.parent)
	return res
}

func (col *allDiskUsageCollector) Metrics() collector.MetricReaderMap {
	name := diskUsagePrefix + diskUsageAll + "/"
	return collector.MetricReaderMap{
		name + "free": col.readFree,
		name + "used": col.readPercent,
	}
}

func (col *allDiskUsageCollector) readFree() (res bitflow.Value) {
	for _, part := range col.parent.partitions {
		res += bitflow.Value(part.stats.Free)
	}
	return
}

func (col *allDiskUsageCollector) readPercent() bitflow.Value {
	var used, total uint64
	for _, part := range col.parent.partitions {
		used += part.stats.Used
		total += part.stats.Total
	}
	if total == 0 {
		return bitflow.Value(0)
	} else {
		return bitflow.Value(used) / bitflow.Value(total) * 100
	}
}
