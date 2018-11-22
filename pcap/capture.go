package pcap

import (
	"io"
)

type CaptureError error

type PacketSource interface {
	Capture() CapturedPacket
}

type CapturedPacket struct {
	Packet Packet
	Size   uint64
	Err    error
}

type Packet interface {
	Info() (*Connection, error)
}

func CapturePackets(sources []PacketSource) <-chan CapturedPacket {
	c := make(chan CapturedPacket)
	for _, source := range sources {
		go func(source PacketSource) {
			for {
				c <- source.Capture()
			}
		}(source)
	}
	return c
}

func (cons *Connections) CapturePackets(sources []PacketSource, errorCallback func(error)) {
	go func() {
		for pkg := range CapturePackets(sources) {
			if pkg.Err != nil {
				errorCallback(pkg.Err)
				if pkg.Err == io.EOF {
					break
				}
			} else {
				con, err := pkg.Packet.Info()
				if err == nil {
					err = cons.LogPacket(con, pkg.Size)
				}
				if err != nil {
					errorCallback(err)
				}
			}
		}
	}()
}
