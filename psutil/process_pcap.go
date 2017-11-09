package psutil

import (
	"errors"
	"io"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/antongulenko/go-bitflow"
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/go-bitflow-collector/pcap/pcap_impl"
)

var (
	PcapNics []string

	pcapStartOnce       = new(sync.Once)
	pcapCons            = pcap.NewConnections()
	globalPcapCollector = pcapCollector{
		AbstractCollector: collector.RootCollector("pcap"),
	}
)

type pcapCollector struct {
	collector.AbstractCollector
}

func (*pcapCollector) Init() ([]collector.Collector, error) {
	if len(PcapNics) == 0 {
		return nil, errors.New("psutil.PcapNics must be set to at least one NIC")
	}
	return nil, pcap_impl.TestLiveCapture(PcapNics)
}

func (*pcapCollector) Update() (err error) {
	pcapStartOnce.Do(func() {
		var sources []pcap.PacketSource
		sources, err = pcap_impl.OpenSources("", PcapNics, true)
		if err == nil {
			pcapCons.CapturePackets(sources, func(err error) {
				if captureErr, ok := err.(pcap.CaptureError); ok {
					log.Debugln("PCAP capture error:", captureErr)
				} else if err == io.EOF {
					// Packet capture is finished, restart on next Update()
					pcapStartOnce = new(sync.Once)
				} else {
					log.Warnln("PCAP capture error:", captureErr)
				}
			})
		}
	})
	if err != nil {
		pcapStartOnce = new(sync.Once)
	}
	return
}

type processPcapCollector struct {
	processSubCollector
}

func (col *PsutilProcessCollector) newProcessPcapCollector() *processPcapCollector {
	result := new(processPcapCollector)
	sub := processSubCollector{
		AbstractCollector: col.AbstractCollector.Child("pcap"),
		parent:            col,
		impl:              result,
	}
	result.processSubCollector = sub
	return result
}

func (col *processPcapCollector) Init() ([]collector.Collector, error) {
	return []collector.Collector{&globalPcapCollector}, nil
}

func (col *processPcapCollector) Depends() []collector.Collector {
	res := col.processSubCollector.Depends()
	res = append(res, &globalPcapCollector)
	return res
}

func (col *processPcapCollector) metrics(parent *PsutilProcessCollector) collector.MetricReaderMap {
	prefix := parent.prefix()
	return collector.MetricReaderMap{
		prefix + "/net-pcap/bytes": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.Bytes.GetDiff()
		}),
		prefix + "/net-pcap/packets": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.Packets.GetDiff()
		}),
		prefix + "/net-pcap/rx_bytes": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.RxBytes.GetDiff()
		}),
		prefix + "/net-pcap/rx_packets": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.RxBytes.GetDiff()
		}),
		prefix + "/net-pcap/tx_bytes": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.TxBytes.GetDiff()
		}),
		prefix + "/net-pcap/tx_packets": parent.sum(func(proc *processInfo) bitflow.Value {
			return proc.net_pcap.TxPackets.GetDiff()
		}),
	}
}

func (col *processPcapCollector) updateProc(info *processInfo) error {
	cons, err := pcapCons.FilterConnections([]int{int(info.Pid)})
	if err != nil {
		return err
	}

	net := &info.net_pcap
	for _, con := range cons {
		net.Bytes.Add(collector.StoredValue(con.RxBytes + con.TxBytes))
		net.RxBytes.Add(collector.StoredValue(con.RxBytes))
		net.TxBytes.Add(collector.StoredValue(con.TxBytes))
		net.Packets.Add(collector.StoredValue(con.RxPackets + con.TxPackets))
		net.RxPackets.Add(collector.StoredValue(con.RxPackets))
		net.TxPackets.Add(collector.StoredValue(con.TxPackets))
	}
	return nil
}
