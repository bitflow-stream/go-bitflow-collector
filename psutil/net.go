package psutil

import (
	"fmt"

	"github.com/antongulenko/go-bitflow-collector"
	psnet "github.com/shirou/gopsutil/net"
)

type PsutilNetCollector struct {
	collector.AbstractCollector
	Factory *collector.ValueRingFactory

	counters NetIoCounters
}

func (col *PsutilNetCollector) Init() error {
	col.Reset(col)
	col.counters = NewNetIoCounters(col.Factory)
	col.Readers = make(map[string]collector.MetricReader)
	col.counters.Register(col.Readers, "net-io")
	return nil
}

func (col *PsutilNetCollector) Update() (err error) {
	counters, err := psnet.IOCounters(false)
	if err == nil && len(counters) != 1 {
		err = fmt.Errorf("gopsutil/net.NetIOCounters() returned %v NetIOCountersStat instead of %v", len(counters), 1)
	}
	if err == nil {
		col.counters.Add(&counters[0])
		col.UpdateMetrics()
	}
	return
}

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

func (counters *BaseNetIoCounters) Register(target map[string]collector.MetricReader, prefix string) {
	target[prefix+"/bytes"] = counters.Bytes.GetDiff
	target[prefix+"/packets"] = counters.Packets.GetDiff
	target[prefix+"/rx_bytes"] = counters.RxBytes.GetDiff
	target[prefix+"/rx_packets"] = counters.RxPackets.GetDiff
	target[prefix+"/tx_bytes"] = counters.TxBytes.GetDiff
	target[prefix+"/tx_packets"] = counters.TxPackets.GetDiff
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

func (counters *NetIoCounters) Register(target map[string]collector.MetricReader, prefix string) {
	counters.BaseNetIoCounters.Register(target, prefix)
	target[prefix+"/errors"] = counters.Errors.GetDiff
	target[prefix+"/dropped"] = counters.Dropped.GetDiff
}
