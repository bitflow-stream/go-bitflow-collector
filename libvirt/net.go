package libvirt

import (
	"fmt"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	lib "github.com/rgbkrk/libvirt-go"
	"gopkg.in/xmlpath.v1"
)

var DomainInterfaceXPath = xmlpath.MustCompile("/domain/devices/interface/target/@dev")

type interfaceStatReader struct {
	parsedInterfaces bool
	interfaces       []string
	net              psutil.NetIoCounters
}

func NewInterfaceStatReader(factory *collector.ValueRingFactory) *interfaceStatReader {
	return &interfaceStatReader{
		net: psutil.NewNetIoCounters(factory),
	}
}

func (reader *interfaceStatReader) description(xmlDesc *xmlpath.Node) {
	reader.interfaces = reader.interfaces[0:0]
	for iter := DomainInterfaceXPath.Iter(xmlDesc); iter.Next(); {
		reader.interfaces = append(reader.interfaces, iter.Node().String())
	}
	reader.parsedInterfaces = true
}

func (reader *interfaceStatReader) register(domainName string) map[string]collector.MetricReader {
	result := make(map[string]collector.MetricReader)
	reader.net.Register(result, "libvirt/"+domainName+"/net-io")
	return result
}

func (reader *interfaceStatReader) update(domain lib.VirDomain) error {
	if !reader.parsedInterfaces {
		return UpdateXmlDescription
	}
	for _, interfaceName := range reader.interfaces {
		// More detailed alternative: domain.GetInterfaceParameters()
		stats, err := domain.InterfaceStats(interfaceName)
		if err == nil {
			reader.net.Bytes.Add(collector.StoredValue(stats.RxBytes + stats.TxBytes))
			reader.net.Packets.Add(collector.StoredValue(stats.RxPackets + stats.TxPackets))
			reader.net.RxBytes.Add(collector.StoredValue(stats.RxBytes))
			reader.net.RxPackets.Add(collector.StoredValue(stats.RxPackets))
			reader.net.TxBytes.Add(collector.StoredValue(stats.TxBytes))
			reader.net.TxPackets.Add(collector.StoredValue(stats.TxPackets))
			reader.net.Errors.Add(collector.StoredValue(stats.RxErrs + stats.TxErrs))
			reader.net.Dropped.Add(collector.StoredValue(stats.RxDrop + stats.TxDrop))
		} else {
			return fmt.Errorf("Failed to update vNIC stats for %s: %v", interfaceName, err)
		}
	}
	return nil
}
