// +build !nopcap

package pcap_impl

import (
	"fmt"
	"strconv"

	"errors"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/google/gopacket"
	pcaplib "github.com/google/gopacket/pcap"
)

const (
	PacketFilter = "tcp or udp"
	SnapLen      = int32(65535)
)

type PacketSource struct {
	*gopacket.PacketSource
}

type Packet struct {
	gopacket.Packet
}

var _ pcap.PacketSource = new(PacketSource)
var _ pcap.Packet = new(Packet)

func PhysicalInterfaces() ([]string, error) {
	nics, err := pcaplib.FindAllDevs()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, nic := range nics {
		for _, addr := range nic.Addresses {
			if addr.IP.IsGlobalUnicast() {
				names = append(names, nic.Name)
				break
			}
		}
	}
	return names, nil
}

func OpenSources(filename string, nics []string, allPublicNics bool) ([]pcap.PacketSource, error) {
	if filename != "" {
		log.Println("Reading packets from", filename)
		source, err := OpenFilePcapDefault(filename)
		return []pcap.PacketSource{source}, err
	}
	if len(nics) == 0 {
		var err error
		nics, err = PhysicalInterfaces()
		if err != nil {
			return nil, err
		}
		if len(nics) == 0 {
			return nil, errors.New("No public NICs found")
		}
		if !allPublicNics {
			nics = nics[0:1]
		}
	}

	log.Println("Capturing packets from", nics)
	res := make([]pcap.PacketSource, len(nics))
	for i, nic := range nics {
		source, err := OpenLivePcapDefault(nic)
		if err != nil {
			return res, err
		}
		res[i] = source
	}
	return res, nil
}

func TestLiveCapture(nics []string) error {
	for _, nic := range nics {
		_, err := OpenLivePcapDefault(nic)
		if err != nil {
			return err
		}
	}
	return nil
}

func OpenFilePcapDefault(filename string) (PacketSource, error) {
	return OpenFilePcap(filename, PacketFilter)
}

func OpenFilePcap(filename string, packetFilter string) (PacketSource, error) {
	if handle, err := pcaplib.OpenOffline(filename); err != nil {
		return PacketSource{}, err
	} else {
		return newPacketSource(handle, packetFilter)
	}
}

func OpenLivePcapDefault(nic string) (PacketSource, error) {
	return OpenLivePcap(nic, PacketFilter, SnapLen)
}

func OpenLivePcap(nic string, packetFilter string, snaplen int32) (PacketSource, error) {
	if handle, err := pcaplib.OpenLive(nic, snaplen, true, pcaplib.BlockForever); err != nil {
		return PacketSource{}, err
	} else {
		return newPacketSource(handle, packetFilter)
	}
}

func newPacketSource(handle *pcaplib.Handle, packetFilter string) (PacketSource, error) {
	if err := handle.SetBPFFilter(packetFilter); err != nil {
		return PacketSource{}, err
	} else {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		packetSource.NoCopy = true
		packetSource.Lazy = true
		return PacketSource{packetSource}, nil
	}
}

func (source PacketSource) Capture() (res pcap.CapturedPacket) {
	var packet gopacket.Packet
	packet, res.Err = source.NextPacket()
	if res.Err != nil {
		return
	}
	if err := packet.ErrorLayer(); err != nil {
		res.Err = pcap.CaptureError(fmt.Errorf("Packet Error: %v", res.Err.Error()))
		return
	}
	res.Size = uint64(packet.Metadata().Length)
	res.Packet = Packet{packet}
	if packet.Metadata().Truncated {
		log.Warnln("pcap.CaptureOnePacket: Packet truncated to len", res.Size)
	}
	return
}

func (packet Packet) Info() (*pcap.Connection, error) {
	var ipType, transportType string
	var srcIp, destIp string
	var srcPort, destPort int
	if transport := packet.TransportLayer(); transport != nil {
		transportType = transport.LayerType().String()
		srcE, destE := transport.TransportFlow().Endpoints()
		srcStr, destStr := srcE.String(), destE.String()
		var srcErr, destErr error
		srcPort, srcErr = strconv.Atoi(srcStr)
		destPort, destErr = strconv.Atoi(destStr)
		if srcErr != nil || destErr != nil {
			return nil, pcap.CaptureError(fmt.Errorf("Illegal port numbers: %v %v", srcStr, destStr))
		}
	} else {
		return nil, nil // Ignore packet without transport layer
	}
	if network := packet.NetworkLayer(); network != nil {
		ipType = network.LayerType().String()
		src, dest := network.NetworkFlow().Endpoints()
		srcIp, destIp = src.String(), dest.String()
	} else {
		return nil, nil // Ignore packet without network layer
	}

	var typ string
	switch transportType {
	case "TCP":
		typ = "tcp"
	case "UDP":
		typ = "udp"
	default:
		return nil, pcap.CaptureError(fmt.Errorf("Illegal transport layer type: %v", transportType))
	}
	switch ipType {
	case "IPv4":
	case "IPv6":
		typ += "6"
	default:
		return nil, pcap.CaptureError(fmt.Errorf("Illegal network layer type: %v", ipType))
	}

	return &pcap.Connection{
		Type:  typ,
		Ip:    srcIp,
		Port:  srcPort,
		Fip:   destIp,
		Fport: destPort,
	}, nil
}
