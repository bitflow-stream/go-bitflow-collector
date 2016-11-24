package psutil

import (
	"errors"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector/pcap"
)

var (
	PcapNics    []string
	PcapSnaplen = int32(65535)

	pcapStartOnce sync.Once
	pcapCons      = pcap.NewConnections()
)

func startPcapCapture() (err error) {
	if len(PcapNics) == 0 {
		return errors.New("psutil.PcapNics must be set to at least one NIC.")
	}

	pcapStartOnce.Do(func() {
		log.Println("Capturing packets from", PcapNics)
		err = pcapCons.CaptureNics(PcapNics, PcapSnaplen, func(err error) {
			if captureErr, ok := err.(pcap.CaptureError); ok {
				log.Warnln("PCAP capture error:", captureErr)
			} else {
				log.Errorln("PCAP capture error:", captureErr)
			}
		})
	})
	return
}

func (self *PsutilProcessCollector) updatePcapNet() error {
	if err := startPcapCapture(); err != nil {
		return err
	}

	pids := make([]int, 0, len(self.procs))
	for pid := range self.procs {
		pids = append(pids, int(pid))
	}
	cons, err := pcapCons.FilterConnections(pids)
	if err != nil {
		return err
	}

	net := &self.net_pcap
	for _, con := range cons {
		net.Bytes.AddValue(bitflow.Value(con.RxBytes + con.TxBytes))
		net.RxBytes.AddValue(bitflow.Value(con.RxBytes))
		net.TxBytes.AddValue(bitflow.Value(con.TxBytes))
		net.Packets.AddValue(bitflow.Value(con.RxPackets + con.TxPackets))
		net.RxPackets.AddValue(bitflow.Value(con.RxPackets))
		net.TxPackets.AddValue(bitflow.Value(con.TxPackets))
	}
	return nil
}
