package psutil

import (
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/disk"
)

type PsutilDiskIOCollector struct {
	collector.AbstractCollector
	Factory *collector.ValueRingFactory

	disks     map[string]disk.IOCountersStat
	diskNames []string
}

func (col *PsutilDiskIOCollector) Init() error {
	col.Reset(col)
	col.disks = make(map[string]disk.IOCountersStat)
	col.diskNames = col.diskNames[:]

	if err := col.update(false); err != nil {
		return err
	}
	col.Readers = make(map[string]collector.MetricReader)
	for disk, _ := range col.disks {
		col.addSimpleReader(disk)
	}
	col.addReader("all", func() []string {
		return col.diskNames
	})
	return nil
}

func (col *PsutilDiskIOCollector) addSimpleReader(disk string) {
	getdisks := func() []string {
		return []string{disk}
	}
	col.addReader(disk, getdisks)
}

func (col *PsutilDiskIOCollector) addReader(name string, getdisks func() []string) {
	name = "disk-io/" + name + "/"
	reader := &diskIOReader{
		col:            col,
		getdisks:       getdisks,
		readRing:       col.Factory.NewValueRing(),
		writeRing:      col.Factory.NewValueRing(),
		ioRing:         col.Factory.NewValueRing(),
		readBytesRing:  col.Factory.NewValueRing(),
		writeBytesRing: col.Factory.NewValueRing(),
		ioBytesRing:    col.Factory.NewValueRing(),
		readTimeRing:   col.Factory.NewValueRing(),
		writeTimeRing:  col.Factory.NewValueRing(),
		ioTimeRing:     col.Factory.NewValueRing(),
	}

	col.Readers[name+"read"] = reader.makeReader(reader.readRing, reader.readRead)
	col.Readers[name+"write"] = reader.makeReader(reader.writeRing, reader.readWrite)
	col.Readers[name+"io"] = reader.makeReader(reader.ioRing, reader.readIo)
	col.Readers[name+"readBytes"] = reader.makeReader(reader.readBytesRing, reader.readReadBytes)
	col.Readers[name+"writeBytes"] = reader.makeReader(reader.writeBytesRing, reader.readWriteBytes)
	col.Readers[name+"ioBytes"] = reader.makeReader(reader.ioBytesRing, reader.readIoBytes)
	col.Readers[name+"readTime"] = reader.makeReader(reader.readTimeRing, reader.readReadTime)
	col.Readers[name+"writeTime"] = reader.makeReader(reader.writeTimeRing, reader.readWriteTime)
	col.Readers[name+"ioTime"] = reader.makeReader(reader.ioTimeRing, reader.readIoTime)
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
	if len(col.diskNames) == 0 {
		for name := range col.disks {
			if !col.isPartition(name) {
				col.diskNames = append(col.diskNames, name)
			}
		}
	}
	return nil
}

func (*PsutilDiskIOCollector) isPartition(name string) bool {
	// TODO very platform specific, but don't see other
	return strings.ContainsRune("0123456789", rune(name[len(name)-1]))
}

func (col *PsutilDiskIOCollector) Update() (err error) {
	if err = col.update(true); err == nil {
		col.UpdateMetrics()
	}
	return
}

type diskIOReader struct {
	col      *PsutilDiskIOCollector
	getdisks func() []string

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

func (reader *diskIOReader) getDisk(diskName string) *disk.IOCountersStat {
	if disk, ok := reader.col.disks[diskName]; ok {
		return &disk
	} else {
		log.Warnf("disk-io counters for disk %v not found", diskName)
		return nil
	}
}

func (reader *diskIOReader) makeReader(ring *collector.ValueRing, getter func(*disk.IOCountersStat) uint64) collector.MetricReader {
	return func() bitflow.Value {
		for _, diskName := range reader.getdisks() {
			if disk := reader.getDisk(diskName); disk != nil {
				val := getter(disk)
				ring.AddToHead(collector.StoredValue(val))
			}
		}
		ring.FlushHead()
		return ring.GetDiff()
	}
}

func (reader *diskIOReader) readRead(d *disk.IOCountersStat) uint64 {
	return d.ReadCount
}

func (reader *diskIOReader) readWrite(d *disk.IOCountersStat) uint64 {
	return d.WriteCount
}

func (reader *diskIOReader) readIo(d *disk.IOCountersStat) uint64 {
	return d.ReadCount + d.WriteCount
}

func (reader *diskIOReader) readReadBytes(d *disk.IOCountersStat) uint64 {
	return d.ReadBytes
}

func (reader *diskIOReader) readWriteBytes(d *disk.IOCountersStat) uint64 {
	return d.WriteBytes
}

func (reader *diskIOReader) readIoBytes(d *disk.IOCountersStat) uint64 {
	return d.ReadBytes + d.WriteBytes
}

func (reader *diskIOReader) readReadTime(d *disk.IOCountersStat) uint64 {
	return d.ReadTime
}

func (reader *diskIOReader) readWriteTime(d *disk.IOCountersStat) uint64 {
	return d.WriteTime
}

func (reader *diskIOReader) readIoTime(d *disk.IOCountersStat) uint64 {
	return d.IoTime
}
