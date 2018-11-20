package ovsdb

import (
	"github.com/bitflow-stream/go-bitflow-collector"
	"github.com/bitflow-stream/go-bitflow-collector/psutil"
)

type ovsdbInterfaceCollector struct {
	collector.AbstractCollector
	parent   *Collector
	counters psutil.NetIoCounters
}

func (parent *Collector) newCollector(name string) *ovsdbInterfaceCollector {
	return &ovsdbInterfaceCollector{
		AbstractCollector: parent.Child(name),
		parent:            parent,
		counters:          psutil.NewNetIoCounters(parent.factory),
	}
}

func (col *ovsdbInterfaceCollector) Metrics() collector.MetricReaderMap {
	return col.counters.Metrics("ovsdb/" + col.Name)
}

func (col *ovsdbInterfaceCollector) Depends() []collector.Collector {
	return []collector.Collector{col.parent}
}

func (col *ovsdbInterfaceCollector) update(stats map[string]float64) {
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

func (col *ovsdbInterfaceCollector) fillValues(stats map[string]float64, names []string, ring *collector.ValueRing) {
	for _, name := range names {
		if value, ok := stats[name]; ok {
			ring.AddToHead(collector.StoredValue(value))
		}
	}
	ring.FlushHead()
}
