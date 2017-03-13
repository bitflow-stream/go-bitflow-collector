package main

import (
	"flag"
	"log"

	"github.com/antongulenko/go-bitflow"
	libpcap "github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/golib"
	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
)

const snaplen = int32(65535)

func main() {
	bitflow.RegisterGolibFlags()
	filename := flag.String("f", "", "PCAP file to parse")
	useNic := flag.Bool("n", false, "Capture packets from local NICs")
	flag.Parse()
	golib.ConfigureLogging()

	var source *gopacket.PacketSource
	if *useNic {
		nics, err := libpcap.PhysicalInterfaces()
		golib.Checkerr(err)
		if len(nics) == 0 {
			golib.Fatalln("No public NICs found")
		}
		nic := nics[0]
		log.Println("Capturing from interface", nic)
		var src libpcap.PacketSource
		src, err = libpcap.OpenPcap(nic, snaplen)
		golib.Checkerr(err)
		source = src.PacketSource
	} else if *filename == "" {
		golib.Fatalln("Please pass -f or -n parameter")
	} else {
		if handle, err := pcap.OpenOffline(*filename); err != nil {
			golib.Fatalln(err)
		} else {
			source = gopacket.NewPacketSource(handle, handle.LinkType())
		}
	}
	for packet := range source.Packets() {
		parsePacket(packet)
	}
}

func parsePacket(packet gopacket.Packet) {
	if app := packet.ApplicationLayer(); app != nil {
		payload := app.Payload()
		sip, err := ParseSipPacket(payload)
		if err != nil {
			log.Println("SIP PARSE ERROR:", err)
		} else if sip == nil {
			log.Println("Not a SIP packet:", packet.TransportLayer().TransportFlow())
		} else {
			log.Println(sip)
		}
	}
}
