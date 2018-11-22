package pcap

import (
	"encoding/binary"
	"fmt"
	"hash/fnv"
)

type Connection struct {
	State string
	Type  string
	Inode string

	Ip    string
	Port  int
	Fip   string
	Fport int

	RxBytes   uint64
	RxPackets uint64
	TxBytes   uint64
	TxPackets uint64
}

func (con *Connection) LogTxPacket(size uint64) {
	con.TxBytes += size
	con.TxPackets++
}

func (con *Connection) LogRxPacket(size uint64) {
	con.RxBytes += size
	con.RxPackets++
}

func (con *Connection) HasData() bool {
	return con.RxPackets+con.TxPackets > 0
}

func (con *Connection) String() string {
	var state, flow, sep, sent, recv string
	if con.State != "" {
		state = fmt.Sprintf(" (%v)", con.State)
	}
	if con.Ip != "" || con.Fip != "" {
		flow = fmt.Sprintf(" %v:%v -> %v:%v", con.Ip, con.Port, con.Fip, con.Fport)
	}
	if con.TxPackets > 0 {
		sent = fmt.Sprintf(" %v packets out (%v byte)", con.TxPackets, con.TxBytes)
		sep = " ="
	}
	if con.RxPackets > 0 {
		recv = fmt.Sprintf(" %v packets in (%v byte)", con.RxPackets, con.RxBytes)
		sep = " ="
	}

	return fmt.Sprintf("%v%v%v%v%v%v", con.Type, state, flow, sep, sent, recv)
}

func (con *Connection) Hash() interface{} {
	return HashConnection(con.Type, con.Ip, con.Port, con.Fip, con.Fport)
}

func (con *Connection) ReverseHash() interface{} {
	return HashConnection(con.Type, con.Fip, con.Fport, con.Ip, con.Port)
}

func HashConnection(typ string, ip string, port int, fip string, fport int) interface{} {
	hash := fnv.New64()
	_, _ = hash.Write([]byte(typ))
	_, _ = hash.Write([]byte(ip))
	_, _ = hash.Write([]byte(fip))
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(port))
	_, _ = hash.Write(b)
	binary.LittleEndian.PutUint32(b, uint32(fport))
	_, _ = hash.Write(b)
	return hash.Sum64()
}
