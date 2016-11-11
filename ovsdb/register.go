package ovsdb

import (
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/socketplane/libovsdb"
)

const (
	OvsdbLogback  = 50
	OvsdbInterval = 5 * time.Second

	DefaultOvsdbPort = libovsdb.DefaultPort
)

func RegisterOvsdbCollector(Host string, factory *collector.ValueRingFactory) {
	RegisterOvsdbCollectorPort(Host, 0, factory)
}

func RegisterOvsdbCollectorPort(host string, port int, factory *collector.ValueRingFactory) {
	collector.RegisterCollector(&OvsdbCollector{Host: host, Port: port, Factory: factory})
}
