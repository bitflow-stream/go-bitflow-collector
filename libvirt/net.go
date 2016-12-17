package libvirt

import (
	"fmt"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	"gopkg.in/xmlpath.v1"
)

var DomainInterfaceXPath = xmlpath.MustCompile("/domain/devices/interface/target/@dev")

type interfaceStatCollector struct {
	vmSubcollectorImpl
	interfaces []string
	net        psutil.NetIoCounters
}

func NewInterfaceStatCollector(parent *vmCollector) *interfaceStatCollector {
	return &interfaceStatCollector{
		vmSubcollectorImpl: parent.child("net-io"),
		net:                psutil.NewNetIoCounters(parent.parent.factory),
	}
}

func (col *interfaceStatCollector) Metrics() collector.MetricReaderMap {
	return col.net.Metrics(col.parent.prefix() + "net-io")
}

func (col *interfaceStatCollector) Update() error {
	for _, interfaceName := range col.interfaces {
		// More detailed alternative: domain.GetInterfaceParameters()
		stats, err := col.parent.domain.InterfaceStats(interfaceName)
		if err != nil {
			return fmt.Errorf("VM %v to update vNIC stats for %s: %v", col.parent.name, interfaceName, err)
		}
		col.net.Bytes.Add(collector.StoredValue(stats.RxBytes + stats.TxBytes))
		col.net.Packets.Add(collector.StoredValue(stats.RxPackets + stats.TxPackets))
		col.net.RxBytes.Add(collector.StoredValue(stats.RxBytes))
		col.net.RxPackets.Add(collector.StoredValue(stats.RxPackets))
		col.net.TxBytes.Add(collector.StoredValue(stats.TxBytes))
		col.net.TxPackets.Add(collector.StoredValue(stats.TxPackets))
		col.net.Errors.Add(collector.StoredValue(stats.RxErrs + stats.TxErrs))
		col.net.Dropped.Add(collector.StoredValue(stats.RxDrop + stats.TxDrop))
	}
	return nil
}

func (col *interfaceStatCollector) description(xmlDesc *xmlpath.Node) {
	col.interfaces = col.interfaces[0:0]
	for iter := DomainInterfaceXPath.Iter(xmlDesc); iter.Next(); {
		col.interfaces = append(col.interfaces, iter.Node().String())
	}
}
