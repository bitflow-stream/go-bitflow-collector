package psutil

import (
	"fmt"
	"regexp"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/disk"
)

// TODO very platform specific
var physicalDiskRegex = regexp.MustCompile("^[sSvV][dD][a-zA-Z]$")

type PsutilDiskIOCollector struct {
	collector.AbstractCollector
	factory *collector.ValueRingFactory
	disks   map[string]disk.IOCountersStat
}

func newDiskIoCollector(root *PsutilRootCollector) *PsutilDiskIOCollector {
	return &PsutilDiskIOCollector{
		AbstractCollector: root.Child("disk"),
		factory:           root.Factory,
	}
}

func (col *PsutilDiskIOCollector) Init() ([]collector.Collector, error) {
	col.disks = make(map[string]disk.IOCountersStat)
	if err := col.update(false); err != nil {
		return nil, err
	}

	res := make([]collector.Collector, 0, len(col.disks)+1)
	allDisks := make([]string, 0, len(col.disks))
	for name := range col.disks {
		if col.isPhysicalDisk(name) {
			allDisks = append(allDisks, name)
			res = append(res, col.newChild(name, []string{name}))
		}
	}
	res = append(res, col.newChild("all", allDisks))
	return res, nil
}

func (col *PsutilDiskIOCollector) Update() error {
	return col.update(true)
}

func (col *PsutilDiskIOCollector) MetricsChanged() error {
	return col.Update()
}

func (col *PsutilDiskIOCollector) newChild(name string, disks []string) *ioDiskCollector {
	return &ioDiskCollector{
		AbstractCollector: col.Child(name),
		parent:            col,
		disks:             disks,

		readRing:       col.factory.NewValueRing(),
		writeRing:      col.factory.NewValueRing(),
		ioRing:         col.factory.NewValueRing(),
		readBytesRing:  col.factory.NewValueRing(),
		writeBytesRing: col.factory.NewValueRing(),
		ioBytesRing:    col.factory.NewValueRing(),
		readTimeRing:   col.factory.NewValueRing(),
		writeTimeRing:  col.factory.NewValueRing(),
		ioTimeRing:     col.factory.NewValueRing(),
	}
}

func (col *PsutilDiskIOCollector) update(checkChange bool) error {
	disks, err := disk.IOCounters()
	if err != nil {
		return err
	}
	if checkChange {
		for k, _ := range col.disks {
			if _, ok := disks[k]; !ok {
				return collector.MetricsChanged
			}
		}
		if len(col.disks) != len(disks) {
			return collector.MetricsChanged
		}
	}
	col.disks = disks
	return nil
}

func (*PsutilDiskIOCollector) isPhysicalDisk(name string) bool {
	return physicalDiskRegex.MatchString(name)
}

type ioDiskCollector struct {
	collector.AbstractCollector
	parent *PsutilDiskIOCollector
	disks  []string

	readRing       *collector.ValueRing
	writeRing      *collector.ValueRing
	ioRing         *collector.ValueRing
	readBytesRing  *collector.ValueRing
	writeBytesRing *collector.ValueRing
	ioBytesRing    *collector.ValueRing
	readTimeRing   *collector.ValueRing
	writeTimeRing  *collector.ValueRing
	ioTimeRing     *collector.ValueRing
}

func (col *ioDiskCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *ioDiskCollector) Update() error {
	for _, diskName := range col.disks {
		d, ok := col.parent.disks[diskName]
		if !ok {
			return fmt.Errorf("disk-io counters for disk %v not found", diskName)
		}
		col.readRing.AddValueToHead(bitflow.Value(d.ReadCount))
		col.writeRing.AddValueToHead(bitflow.Value(d.WriteCount))
		col.ioRing.AddValueToHead(bitflow.Value(d.ReadCount + d.WriteCount))
		col.readBytesRing.AddValueToHead(bitflow.Value(d.ReadBytes))
		col.writeBytesRing.AddValueToHead(bitflow.Value(d.WriteBytes))
		col.ioBytesRing.AddValueToHead(bitflow.Value(d.ReadBytes + d.WriteBytes))
		col.readTimeRing.AddValueToHead(bitflow.Value(d.ReadTime))
		col.writeTimeRing.AddValueToHead(bitflow.Value(d.WriteTime))
		col.ioTimeRing.AddValueToHead(bitflow.Value(d.IoTime))
	}
	col.readRing.FlushHead()
	col.writeRing.FlushHead()
	col.ioRing.FlushHead()
	col.readBytesRing.FlushHead()
	col.writeBytesRing.FlushHead()
	col.ioBytesRing.FlushHead()
	col.readTimeRing.FlushHead()
	col.writeTimeRing.FlushHead()
	col.ioTimeRing.FlushHead()
	return nil
}

func (col *ioDiskCollector) Metrics() collector.MetricReaderMap {
	name := "disk-io/" + col.Name + "/"
	return collector.MetricReaderMap{
		name + "read":       col.readRing.GetDiff,
		name + "write":      col.writeRing.GetDiff,
		name + "io":         col.ioRing.GetDiff,
		name + "readBytes":  col.readBytesRing.GetDiff,
		name + "writeBytes": col.writeBytesRing.GetDiff,
		name + "ioBytes":    col.ioBytesRing.GetDiff,
		name + "readTime":   col.readTimeRing.GetDiff,
		name + "writeTime":  col.writeTimeRing.GetDiff,
		name + "ioTime":     col.ioTimeRing.GetDiff,
	}
}
