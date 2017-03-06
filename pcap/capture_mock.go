// +build !pcap

package pcap

var (
	mock_interface   = "mock"
	mock_packet_size = uint64(128)
	mock_packet_info = &Connection{
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

type PacketSource int

func PhysicalInterfaces() ([]string, error) {
	return []string{mock_interface}, nil
}

func OpenPcap(nic string, snaplen int32) (PacketSource, error) {
	return PacketSource(1), nil
}

func CaptureOnePacket(source PacketSource, connections *Connections) error {
	return connections.LogPacket(mock_packet_info, mock_packet_size)
}
