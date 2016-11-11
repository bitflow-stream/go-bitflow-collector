package psutil

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/disk"
)

type PsutilDiskUsageCollector struct {
	collector.AbstractCollector
	allPartitions      map[string]bool
	observedPartitions map[string]bool
	usage              map[string]*disk.UsageStat
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
	for partition, _ := range col.allPartitions {
		name := "disk-usage/" + partition + "/"
		reader := &diskUsageReader{
			col:       col,
			partition: partition,
		}
		col.Readers[name+"free"] = reader.readFree
		col.Readers[name+"used"] = reader.readPercent
	}
	return nil
}

func (col *PsutilDiskUsageCollector) Collect(metric *collector.Metric) error {
	lastSlash := strings.LastIndex(metric.Name, "/")
	partition := metric.Name[len("disk-usage/"):lastSlash]
	col.observedPartitions[partition] = true
	return col.AbstractCollector.Collect(metric)
}

func (col *PsutilDiskUsageCollector) getAllPartitions() (map[string]bool, error) {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return nil, err
	}
	result := make(map[string]bool, len(partitions))
	for _, partition := range partitions {
		result[partition.Mountpoint] = true
	}
	return result, nil
}

func (col *PsutilDiskUsageCollector) update() error {
	for partition, _ := range col.observedPartitions {
		usage, err := disk.Usage(partition)
		if err != nil {
			return err
		}
		col.usage[partition] = usage
	}
	return nil
}

func (col *PsutilDiskUsageCollector) checkChangedPartitions() error {
	partitions, err := disk.Partitions(true)
	if err != nil {
		return err
	}
	checked := make(map[string]bool, len(partitions))
	for _, partition := range partitions {
		if _, ok := col.allPartitions[partition.Mountpoint]; !ok {
			return collector.MetricsChanged
		}
		checked[partition.Mountpoint] = true
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

type diskUsageReader struct {
	col       *PsutilDiskUsageCollector
	partition string
}

func (reader *diskUsageReader) checkDisk() *disk.UsageStat {
	if disk, ok := reader.col.usage[reader.partition]; ok {
		return disk
	} else {
		log.Warnf("disk-usage counters for partition %v not found", reader.partition)
		return nil
	}
}

func (reader *diskUsageReader) readFree() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return bitflow.Value(disk.Free)
	}
	return bitflow.Value(0)
}

func (reader *diskUsageReader) readPercent() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return bitflow.Value(disk.UsedPercent)
	}
	return bitflow.Value(0)
}
