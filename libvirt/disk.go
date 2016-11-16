package libvirt

import (
	"fmt"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

var DomainBlockXPath = xmlpath.MustCompile("/domain/devices/disk[@type=\"file\"]/target/@dev")

type blockStatReader struct {
	parsedDevices bool
	devices       []string
	info          []lib.VirDomainBlockInfo
	stats         []lib.VirDomainBlockStats

	ioRing      *collector.ValueRing
	ioBytesRing *collector.ValueRing
}

func NewBlockStatReader(factory *collector.ValueRingFactory) *blockStatReader {
	return &blockStatReader{
		ioRing:      factory.NewValueRing(),
		ioBytesRing: factory.NewValueRing(),
	}
}

func (reader *blockStatReader) register(domainName string) map[string]collector.MetricReader {
	return map[string]collector.MetricReader{
		"libvirt/" + domainName + "/block/allocation": reader.readAllocation,
		"libvirt/" + domainName + "/block/capacity":   reader.readCapacity,
		"libvirt/" + domainName + "/block/physical":   reader.readPhysical,
		"libvirt/" + domainName + "/block/io":         reader.readIo,
		"libvirt/" + domainName + "/block/ioBytes":    reader.readIoBytes,
	}
}

func (reader *blockStatReader) description(xmlDesc *xmlpath.Node) {
	reader.devices = reader.devices[:]
	for iter := DomainBlockXPath.Iter(xmlDesc); iter.Next(); {
		reader.devices = append(reader.devices, iter.Node().String())
	}
	reader.parsedDevices = true
}

func (reader *blockStatReader) update(domain lib.VirDomain) error {
	reader.info = reader.info[:]
	if !reader.parsedDevices {
		return UpdateXmlDescription
	}
	var resErr error
	for _, dev := range reader.devices {
		if info, err := domain.GetBlockInfo(dev, NO_FLAGS); err == nil {
			reader.info = append(reader.info, info)
		} else {
			return fmt.Errorf("Failed to get block-device info for %s: %v", dev, err)
		}

		// More detailed alternative: domain.BlockStatsFlags()
		if stats, err := domain.BlockStats(dev); err == nil {
			reader.stats = append(reader.stats, stats)
		} else {
			return fmt.Errorf("Failed to get block-device stats for %s: %v", dev, err)
		}
	}
	return resErr
}

func (reader *blockStatReader) readAllocation() (result bitflow.Value) {
	for _, info := range reader.info {
		result += bitflow.Value(info.Allocation())
	}
	return
}

func (reader *blockStatReader) readCapacity() (result bitflow.Value) {
	for _, info := range reader.info {
		result += bitflow.Value(info.Capacity())
	}
	return
}

func (reader *blockStatReader) readPhysical() (result bitflow.Value) {
	for _, info := range reader.info {
		result += bitflow.Value(info.Physical())
	}
	return
}

func (reader *blockStatReader) readIo() bitflow.Value {
	var result bitflow.Value
	for _, stats := range reader.stats {
		result += bitflow.Value(stats.RdReq + stats.WrReq)
	}
	reader.ioRing.AddValue(result)
	return reader.ioRing.GetDiff()
}

func (reader *blockStatReader) readIoBytes() bitflow.Value {
	var result bitflow.Value
	for _, stats := range reader.stats {
		result += bitflow.Value(stats.RdBytes + stats.WrBytes)
	}
	reader.ioBytesRing.AddValue(result)
	return reader.ioBytesRing.GetDiff()
}
