package pcap

import (
	"io"

	log "github.com/Sirupsen/logrus"
)

const PacketFilter = "tcp or udp"

type CaptureError error

func (cons *Connections) CaptureNics(nics []string, snaplen int32, errorCallback func(error)) error {
	sources := make([]PacketSource, 0, len(nics))
	for _, nic := range nics {
		source, err := OpenPcap(nic, snaplen)
		if err != nil {
			return err
		}
		sources = append(sources, source)
	}
	log.Println("Capturing packets from", nics)
	for _, source := range sources {
		go func(source PacketSource) {
			for {
				err := CaptureOnePacket(source, cons)
				if err != nil {
					errorCallback(err)
					if err == io.EOF {
						break
					}
				}
			}
		}(source)
	}
	return nil
}

func TestCapture(nics []string, snaplen int32) error {
	for _, nic := range nics {
		_, err := OpenPcap(nic, snaplen)
		if err != nil {
			return err
		}
	}
	return nil
}
