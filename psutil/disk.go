package psutil

import (
	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/shirou/gopsutil/disk"
)

type PsutilDiskIOCollector struct {
	collector.AbstractCollector
	Factory *collector.ValueRingFactory

	disks map[string]disk.IOCountersStat
}

func (col *PsutilDiskIOCollector) Init() error {
	col.Reset(col)
	col.disks = make(map[string]disk.IOCountersStat)

	if err := col.update(false); err != nil {
		return err
	}
	col.Readers = make(map[string]collector.MetricReader)
	for disk, _ := range col.disks {
		name := "disk-io/" + disk + "/"
		reader := &diskIOReader{
			col:            col,
			disk:           disk,
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
		col.Readers[name+"read"] = reader.readRead
		col.Readers[name+"write"] = reader.readWrite
		col.Readers[name+"io"] = reader.readIo
		col.Readers[name+"readBytes"] = reader.readReadBytes
		col.Readers[name+"writeBytes"] = reader.readWriteBytes
		col.Readers[name+"ioBytes"] = reader.readIoBytes
		col.Readers[name+"readTime"] = reader.readReadTime
		col.Readers[name+"writeTime"] = reader.readWriteTime
		col.Readers[name+"ioTime"] = reader.readIoTime
	}
	return nil
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

func (col *PsutilDiskIOCollector) Update() (err error) {
	if err = col.update(true); err == nil {
		col.UpdateMetrics()
	}
	return
}

type diskIOReader struct {
	col  *PsutilDiskIOCollector
	disk string

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

func (reader *diskIOReader) checkDisk() *disk.IOCountersStat {
	if disk, ok := reader.col.disks[reader.disk]; ok {
		return &disk
	} else {
		log.Warnf("disk-io counters for disk %v not found", reader.disk)
		return nil
	}
}

func (reader *diskIOReader) value(val uint64, ring *collector.ValueRing) bitflow.Value {
	ring.Add(collector.StoredValue(val))
	return ring.GetDiff()
}

func (reader *diskIOReader) readRead() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.ReadCount, reader.readRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readWrite() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.WriteCount, reader.writeRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readIo() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.ReadCount+disk.WriteCount, reader.ioRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readReadBytes() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.ReadBytes, reader.readBytesRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readWriteBytes() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.WriteBytes, reader.writeBytesRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readIoBytes() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.ReadBytes+disk.WriteBytes, reader.ioBytesRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readReadTime() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.ReadTime, reader.readTimeRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readWriteTime() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.WriteTime, reader.writeTimeRing)
	}
	return bitflow.Value(0)
}

func (reader *diskIOReader) readIoTime() bitflow.Value {
	if disk := reader.checkDisk(); disk != nil {
		return reader.value(disk.IoTime, reader.ioTimeRing)
	}
	return bitflow.Value(0)
}
