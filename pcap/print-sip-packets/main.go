// +build !nopcap

package main

import (
	"flag"
	"log"

	"io"

	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/go-bitflow-collector/pcap/pcap_impl"
	"github.com/antongulenko/golib"
	"github.com/google/gopacket"
)

const snaplen = int32(65535)

func main() {
	bitflow.RegisterGolibFlags()
	filename := flag.String("f", "", "PCAP file to parse")
	flag.Parse()
	golib.ConfigureLogging()

	sources, err := pcap_impl.OpenSources(*filename, nil, true)
	golib.Checkerr(err)
	c := pcap.CapturePackets(sources)
	for packet := range c {
		if packet.Err != nil {
			if packet.Err == io.EOF {
				break
			} else {
				log.Println(packet.Err)
			}
		} else {
			parsePacket(packet.Packet.(pcap_impl.Packet).Packet)
		}
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
