package libvirt

import (
	"fmt"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

var DomainBlockXPath = xmlpath.MustCompile("/domain/devices/disk[@type=\"file\"]/target/@dev")

type vmBlockCollector struct {
	vmSubcollectorImpl
	devices []string
}

func NewBlockCollector(parent *vmCollector) *vmBlockCollector {
	return &vmBlockCollector{
		vmSubcollectorImpl: parent.child("block"),
	}
}

func (col *vmBlockCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{
		&vmBlockIoCollector{
			AbstractCollector: col.Child("block-io"),
			parent:            col,
		},
		&vmBlockStatsCollector{
			AbstractCollector: col.Child("block-stats"),
			parent:            col,
		},
	}, nil
}

func (col *vmBlockCollector) description(xmlDesc *xmlpath.Node) {
	col.devices = col.devices[0:0]
	for iter := DomainBlockXPath.Iter(xmlDesc); iter.Next(); {
		col.devices = append(col.devices, iter.Node().String())
	}
}

// ===================================== block io =====================================

type vmBlockIoCollector struct {
	collector.AbstractCollector
	parent      *vmBlockCollector
	stats       []lib.VirDomainBlockStats
	ioRing      *collector.ValueRing
	ioBytesRing *collector.ValueRing
}

func (col *vmBlockIoCollector) Init() ([]collector.Collector, error) {
	factory := col.parent.parent.parent.factory
	col.ioRing = factory.NewValueRing()
	col.ioBytesRing = factory.NewValueRing()
	return nil, nil
}

func (col *vmBlockIoCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "block/io":      col.readIo,
		prefix + "block/ioBytes": col.readIoBytes,
	}
}

func (col *vmBlockIoCollector) Update() error {
	new_stats := make([]lib.VirDomainBlockStats, 0, len(col.parent.devices))
	for _, dev := range col.parent.devices {
		// More detailed alternative: domain.BlockStatsFlags()
		if block_stats, err := col.parent.parent.domain.BlockStats(dev); err == nil {
			new_stats = append(new_stats, block_stats)
		} else {
			return fmt.Errorf("Failed to get block-device stats for %s: %v", dev, err)
		}
	}
	col.stats = new_stats
	return nil
}

func (col *vmBlockIoCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *vmBlockIoCollector) readIo() bitflow.Value {
	var result bitflow.Value
	for _, stats := range col.stats {
		result += bitflow.Value(stats.RdReq + stats.WrReq)
	}
	col.ioRing.AddValue(result)
	return col.ioRing.GetDiff()
}

func (col *vmBlockIoCollector) readIoBytes() bitflow.Value {
	var result bitflow.Value
	for _, stats := range col.stats {
		result += bitflow.Value(stats.RdBytes + stats.WrBytes)
	}
	col.ioBytesRing.AddValue(result)
	return col.ioBytesRing.GetDiff()
}

// ===================================== block usage =====================================

type vmBlockStatsCollector struct {
	collector.AbstractCollector
	parent *vmBlockCollector
	info   []lib.VirDomainBlockInfo
}

func (col *vmBlockStatsCollector) Metrics() collector.MetricReaderMap {
	prefix := col.parent.parent.prefix()
	return collector.MetricReaderMap{
		prefix + "block/allocation": col.readAllocation,
		prefix + "block/capacity":   col.readCapacity,
		prefix + "block/physical":   col.readPhysical,
	}
}

func (col *vmBlockStatsCollector) Update() error {
	new_info := make([]lib.VirDomainBlockInfo, 0, len(col.parent.devices))
	for _, dev := range col.parent.devices {
		if block_info, err := col.parent.parent.domain.BlockInfo(dev); err == nil {
			new_info = append(new_info, block_info)
		} else {
			return fmt.Errorf("Failed to get block-device info for %s: %v", dev, err)
		}
	}
	col.info = new_info
	return nil
}

func (col *vmBlockStatsCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *vmBlockStatsCollector) readAllocation() (result bitflow.Value) {
	for _, info := range col.info {
		result += bitflow.Value(info.Allocation())
	}
	return
}

func (col *vmBlockStatsCollector) readCapacity() (result bitflow.Value) {
	for _, info := range col.info {
		result += bitflow.Value(info.Capacity())
	}
	return
}

func (col *vmBlockStatsCollector) readPhysical() (result bitflow.Value) {
	for _, info := range col.info {
		result += bitflow.Value(info.Physical())
	}
	return
}
