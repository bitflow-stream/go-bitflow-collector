// +build nopcap

package pcap_impl

import (
	"log"

	"github.com/antongulenko/go-bitflow-collector/pcap"
)

var (
	mock_packet_size = uint64(128)
	mock_packet_info = &pcap.Connection{
		State: "connected",
		Type:  "tcp",
		Inode: "123",

		Ip:    "192.168.1.1",
		Port:  50000,
		Fip:   "192.168.1.100",
		Fport: 80,

		RxBytes:   100,
		RxPackets: 20,
		TxBytes:   100,
		TxPackets: 20,
	}
)

type (
	PacketSource int
	Packet       int
)

var (
	packetSource = PacketSource(0)
	packet       = Packet(0)
)

func OpenSources(filename string, nics []string, _ bool) ([]pcap.PacketSource, error) {
	if filename != "" {
		log.Println("Reading mock packets from file", filename)
		return []pcap.PacketSource{packetSource}, nil
	}
	if len(nics) == 0 {
		nics = []string{"mock_nic"}
	}

	log.Println("Capturing mocck packets from interface(s):", nics)
	res := make([]pcap.PacketSource, len(nics))
	for i := range nics {
		res[i] = packetSource
	}
	return res, nil
}

func TestLiveCapture(_ []string) error {
	return nil
}

func (source PacketSource) Capture() (res pcap.CapturedPacket) {
	res.Size = mock_packet_size
	res.Packet = packet
	return
}

func (packet Packet) Info() (*pcap.Connection, error) {
	return mock_packet_info, nil
}
