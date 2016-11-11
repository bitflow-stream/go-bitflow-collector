package ovsdb

import (
	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/psutil"
)

type ovsdbInterfaceReader struct {
	name     string
	col      *OvsdbCollector
	counters psutil.NetIoCounters
}

func (col *ovsdbInterfaceReader) fillValues(stats map[string]float64, names []string, ring *collector.ValueRing) {
	for _, name := range names {
		if value, ok := stats[name]; ok {
			ring.AddToHead(collector.StoredValue(value))
		}
	}
	ring.FlushHead()
}

func (col *ovsdbInterfaceReader) update(stats map[string]float64) {
	col.fillValues(stats, []string{
		"collisions",
		"rx_crc_err",
		"rx_errors",
		"rx_frame_err",
		"rx_over_err",
		"tx_errors",
	}, col.counters.Errors)
	col.fillValues(stats, []string{"rx_dropped", "tx_dropped"}, col.counters.Dropped)
	col.fillValues(stats, []string{"rx_bytes", "tx_bytes"}, col.counters.Bytes)
	col.fillValues(stats, []string{"rx_packets", "tx_packets"}, col.counters.Packets)
	col.fillValues(stats, []string{"rx_bytes"}, col.counters.RxBytes)
	col.fillValues(stats, []string{"rx_packets"}, col.counters.RxPackets)
	col.fillValues(stats, []string{"tx_bytes"}, col.counters.TxBytes)
	col.fillValues(stats, []string{"tx_packets"}, col.counters.TxPackets)
}
