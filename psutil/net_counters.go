package psutil

import (
	"github.com/bitflow-stream/go-bitflow-collector"
	psnet "github.com/shirou/gopsutil/net"
)

type BaseNetIoCounters struct {
	Bytes     *collector.ValueRing
	Packets   *collector.ValueRing
	RxBytes   *collector.ValueRing
	RxPackets *collector.ValueRing
	TxBytes   *collector.ValueRing
	TxPackets *collector.ValueRing
}

func NewBaseNetIoCounters(factory *collector.ValueRingFactory) BaseNetIoCounters {
	return BaseNetIoCounters{
		Bytes:     factory.NewValueRing(),
		Packets:   factory.NewValueRing(),
		RxBytes:   factory.NewValueRing(),
		RxPackets: factory.NewValueRing(),
		TxBytes:   factory.NewValueRing(),
		TxPackets: factory.NewValueRing(),
	}
}

func (counters *BaseNetIoCounters) Add(stat *psnet.IOCountersStat) {
	counters.AddToHead(stat)
	counters.FlushHead()
}

func (counters *BaseNetIoCounters) AddToHead(stat *psnet.IOCountersStat) {
	counters.Bytes.AddToHead(collector.StoredValue(stat.BytesSent + stat.BytesRecv))
	counters.Packets.AddToHead(collector.StoredValue(stat.PacketsSent + stat.PacketsRecv))
	counters.RxBytes.AddToHead(collector.StoredValue(stat.BytesRecv))
	counters.RxPackets.AddToHead(collector.StoredValue(stat.PacketsRecv))
	counters.TxBytes.AddToHead(collector.StoredValue(stat.BytesSent))
	counters.TxPackets.AddToHead(collector.StoredValue(stat.PacketsSent))
}

func (counters *BaseNetIoCounters) FlushHead() {
	counters.Bytes.FlushHead()
	counters.Packets.FlushHead()
	counters.RxBytes.FlushHead()
	counters.RxPackets.FlushHead()
	counters.TxBytes.FlushHead()
	counters.TxPackets.FlushHead()
}

func (counters *BaseNetIoCounters) Metrics(prefix string) collector.MetricReaderMap {
	return collector.MetricReaderMap{
		prefix + "/bytes":      counters.Bytes.GetDiff,
		prefix + "/packets":    counters.Packets.GetDiff,
		prefix + "/rx_bytes":   counters.RxBytes.GetDiff,
		prefix + "/rx_packets": counters.RxPackets.GetDiff,
		prefix + "/tx_bytes":   counters.TxBytes.GetDiff,
		prefix + "/tx_packets": counters.TxPackets.GetDiff,
	}
}

type NetIoCounters struct {
	BaseNetIoCounters
	Errors  *collector.ValueRing
	Dropped *collector.ValueRing
}

func NewNetIoCounters(factory *collector.ValueRingFactory) NetIoCounters {
	return NetIoCounters{
		BaseNetIoCounters: NewBaseNetIoCounters(factory),
		Errors:            factory.NewValueRing(),
		Dropped:           factory.NewValueRing(),
	}
}

func (counters *NetIoCounters) Add(stat *psnet.IOCountersStat) {
	counters.AddToHead(stat)
	counters.FlushHead()
}

func (counters *NetIoCounters) AddToHead(stat *psnet.IOCountersStat) {
	counters.BaseNetIoCounters.AddToHead(stat)
	counters.Errors.AddToHead(collector.StoredValue(stat.Errin + stat.Errout))
	counters.Dropped.AddToHead(collector.StoredValue(stat.Dropin + stat.Dropout))
}

func (counters *NetIoCounters) FlushHead() {
	counters.BaseNetIoCounters.FlushHead()
	counters.Errors.FlushHead()
	counters.Dropped.FlushHead()
}

func (counters *NetIoCounters) Metrics(prefix string) collector.MetricReaderMap {
	m := counters.BaseNetIoCounters.Metrics(prefix)
	m[prefix+"/errors"] = counters.Errors.GetDiff
	m[prefix+"/dropped"] = counters.Dropped.GetDiff
	return m
}
