package pcap

import (
	"fmt"
	"io"
	"strconv"

	log "github.com/Sirupsen/logrus"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

const PacketFilter = "tcp or udp"

type CaptureError error

func PhysicalInterfaces() ([]string, error) {
	nics, err := pcap.FindAllDevs()
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

func OpenPcap(nic string, snaplen int32) (*gopacket.PacketSource, error) {
	if handle, err := pcap.OpenLive(nic, snaplen, true, pcap.BlockForever); err != nil {
		return nil, err
	} else if err := handle.SetBPFFilter(PacketFilter); err != nil {
		return nil, err
	} else {
		packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
		packetSource.NoCopy = true
		packetSource.Lazy = true
		return packetSource, nil
	}
}

func CaptureOnePacket(source *gopacket.PacketSource, connections *Connections) error {
	packet, err := source.NextPacket()
	if err != nil {
		return err
	}
	if err := packet.ErrorLayer(); err != nil {
		return CaptureError(fmt.Errorf("Packet Error: %v", err.Error()))
	}
	info, err := getConnectionInfo(packet)
	if err != nil || info == nil {
		return err
	}
	size := uint64(packet.Metadata().Length)
	if packet.Metadata().Truncated {
		log.Warnln("pcap.CaptureOnePacket: Packet truncated to len", size)
	}

	return connections.LogPacket(info, size)
}

func (cons *Connections) CaptureNics(nics []string, snaplen int32, errorCallback func(error)) error {
	sources := make([]*gopacket.PacketSource, 0, len(nics))
	for _, nic := range nics {
		source, err := OpenPcap(nic, snaplen)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}
	for _, source := range sources {
		go func() {
			for {
				err := CaptureOnePacket(source, cons)
				if err != nil {
					errorCallback(err)
					if err == io.EOF {
						break
					}
				}
			}
		}()
	}
	return nil
}

func getConnectionInfo(packet gopacket.Packet) (*Connection, error) {
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
			return nil, CaptureError(fmt.Errorf("Illegal port numbers:", srcStr, destStr))
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
		return nil, CaptureError(fmt.Errorf("Illegal transport layer type: %v", transportType))
	}
	switch ipType {
	case "IPv4":
	case "IPv6":
		typ += "6"
	default:
		return nil, CaptureError(fmt.Errorf("Illegal network layer type: %v", ipType))
	}

	return &Connection{
		Type:  typ,
		Ip:    srcIp,
		Port:  srcPort,
		Fip:   destIp,
		Fport: destPort,
	}, nil
}
