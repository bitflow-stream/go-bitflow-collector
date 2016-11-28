package ovsdb

import (
	"fmt"
	"sync"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/socketplane/libovsdb"
)

const DefaultOvsdbPort = libovsdb.DefaultPort

type OvsdbCollector struct {
	collector.AbstractCollector
	Host    string
	Port    int
	factory *collector.ValueRingFactory

	client              *libovsdb.OvsdbClient
	lastUpdateError     error
	notifier            ovsdbNotifier
	interfaceCollectors map[string]*ovsdbInterfaceCollector
	readersLock         sync.Mutex
}

func NewOvsdbCollector(host string, factory *collector.ValueRingFactory) *OvsdbCollector {
	return NewOvsdbCollectorPort(host, 0, factory)
}

func NewOvsdbCollectorPort(host string, port int, factory *collector.ValueRingFactory) *OvsdbCollector {
	return &OvsdbCollector{
		AbstractCollector: collector.RootCollector("ovsdb"),
		Host:              host,
		Port:              port,
		factory:           factory}
}

func (col *OvsdbCollector) Init() ([]collector.Collector, error) {
	col.Close()
	col.notifier.col = col
	col.lastUpdateError = nil
	col.interfaceCollectors = make(map[string]*ovsdbInterfaceCollector)
	if err := col.update(false); err != nil {
		return nil, err
	}

	readers := make([]collector.Collector, 0, len(col.interfaceCollectors))
	for _, reader := range col.interfaceCollectors {
		readers = append(readers, reader)
	}
	return readers, nil
}

func (col *OvsdbCollector) Update() error {
	return col.update(true)
}

func (col *OvsdbCollector) MetricsChanged() error {
	return col.Update()
}

func (col *OvsdbCollector) Close() {
	if client := col.client; client != nil {
		client.Disconnect()
		col.client = nil
	}
}

func (col *OvsdbCollector) update(checkChange bool) error {
	if col.lastUpdateError != nil {
		col.Close()
		return col.lastUpdateError
	}
	if err := col.ensureConnection(checkChange); err != nil {
		return err
	}
	return nil
}

func (col *OvsdbCollector) ensureConnection(checkChange bool) error {
	if col.client == nil {
		initialTables, ovs, err := col.openConnection()
		if err == nil {
			col.client = ovs
			return col.updateTables(checkChange, initialTables.Updates)
		} else {
			return err
		}
	}
	return nil
}

func (col *OvsdbCollector) openConnection() (*libovsdb.TableUpdates, *libovsdb.OvsdbClient, error) {
	ovs, err := libovsdb.Connect(col.Host, col.Port)
	if err != nil {
		return nil, nil, err
	}
	ovs.Register(&col.notifier)

	// Request all updates for all Interface statistics
	requests := map[string]libovsdb.MonitorRequest{
		"Interface": libovsdb.MonitorRequest{
			Columns: []string{"name", "statistics"},
		},
	}

	initial, err := ovs.Monitor("Open_vSwitch", "", requests)
	if err != nil {
		ovs.Disconnect()
		return nil, nil, err
	}
	return initial, ovs, nil
}

func (col *OvsdbCollector) updateTables(checkChange bool, updates map[string]libovsdb.TableUpdate) error {
	update, ok := updates["Interface"]
	if !ok {
		return fmt.Errorf("OVSDB update did not contain requested table 'Interface'. Instead: %v", updates)
	}

	col.readersLock.Lock()
	defer col.readersLock.Unlock()
	updatedInterfaces := make(map[string]bool)
	for _, rowUpdate := range update.Rows {
		if name, stats, err := col.parseRowUpdate(rowUpdate.New); err != nil {
			return err
		} else {
			updatedInterfaces[name] = true
			reader, ok := col.interfaceCollectors[name]
			if !ok {
				if checkChange {
					return collector.MetricsChanged
				} else {
					reader = col.newCollector(name)
					col.interfaceCollectors[name] = reader
				}
			}
			reader.update(stats)
		}
	}

	// Assume that every update includes info about all interfaces
	if checkChange && len(updatedInterfaces) != len(col.interfaceCollectors) {
		return collector.MetricsChanged
	}
	return nil
}

func (col *OvsdbCollector) parseRowUpdate(row libovsdb.Row) (name string, stats map[string]float64, err error) {
	defer func() {
		// Allow panics for less explicit type checks
		if rec := recover(); rec != nil {
			err = fmt.Errorf("Parsing OVSDB row updated failed: %v", rec)
		}
	}()

	if nameObj, ok := row.Fields["name"]; !ok {
		err = fmt.Errorf("Row update did not include 'name' field")
		return
	} else {
		name = nameObj.(string)
	}
	if statsObj, ok := row.Fields["statistics"]; !ok {
		err = fmt.Errorf("Row update did not include 'statistics' field")
	} else {
		statMap := statsObj.(libovsdb.OvsMap)
		stats = make(map[string]float64)
		for keyObj, valObj := range statMap.GoMap {
			stats[keyObj.(string)] = valObj.(float64)
		}
	}
	return
}
