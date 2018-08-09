package ovsdb

import (
	"errors"
	"fmt"
	"sync"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/socketplane/libovsdb"
)

const DefaultOvsdbPort = libovsdb.DefaultPort

type Collector struct {
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

func NewOvsdbCollector(host string, factory *collector.ValueRingFactory) *Collector {
	return NewOvsdbCollectorPort(host, 0, factory)
}

func NewOvsdbCollectorPort(host string, port int, factory *collector.ValueRingFactory) *Collector {
	return &Collector{
		AbstractCollector: collector.RootCollector("ovsdb"),
		Host:              host,
		Port:              port,
		factory:           factory}
}

func (parent *Collector) Init() ([]collector.Collector, error) {
	parent.Close()
	parent.notifier.col = parent
	parent.lastUpdateError = nil
	parent.interfaceCollectors = make(map[string]*ovsdbInterfaceCollector)
	if err := parent.update(false); err != nil {
		return nil, err
	}

	readers := make([]collector.Collector, 0, len(parent.interfaceCollectors))
	for _, reader := range parent.interfaceCollectors {
		readers = append(readers, reader)
	}
	return readers, nil
}

func (parent *Collector) Update() error {
	return parent.update(true)
}

func (parent *Collector) MetricsChanged() error {
	return parent.Update()
}

func (parent *Collector) Close() {
	if client := parent.client; client != nil {
		client.Disconnect()
		parent.client = nil
	}
}

func (parent *Collector) update(checkChange bool) error {
	if parent.lastUpdateError != nil {
		parent.Close()
		return parent.lastUpdateError
	}
	if err := parent.ensureConnection(checkChange); err != nil {
		return err
	}
	return nil
}

func (parent *Collector) ensureConnection(checkChange bool) error {
	if parent.client == nil {
		initialTables, ovs, err := parent.openConnection()
		if err == nil {
			parent.client = ovs
			return parent.updateTables(checkChange, initialTables.Updates)
		} else {
			return err
		}
	}
	return nil
}

func (parent *Collector) openConnection() (*libovsdb.TableUpdates, *libovsdb.OvsdbClient, error) {
	ovs, err := libovsdb.Connect(parent.Host, parent.Port)
	if err != nil {
		return nil, nil, err
	}
	ovs.Register(&parent.notifier)

	// Request all updates for all Interface statistics
	requests := map[string]libovsdb.MonitorRequest{
		"Interface": {
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

func (parent *Collector) updateTables(checkChange bool, updates map[string]libovsdb.TableUpdate) error {
	update, ok := updates["Interface"]
	if !ok {
		return fmt.Errorf("OVSDB update did not contain requested table 'Interface'. Instead: %v", updates)
	}

	parent.readersLock.Lock()
	defer parent.readersLock.Unlock()
	updatedInterfaces := make(map[string]bool)
	for _, rowUpdate := range update.Rows {
		if name, stats, err := parent.parseRowUpdate(rowUpdate.New); err != nil {
			return err
		} else {
			updatedInterfaces[name] = true
			reader, ok := parent.interfaceCollectors[name]
			if !ok {
				if checkChange {
					return collector.MetricsChanged
				} else {
					reader = parent.newCollector(name)
					parent.interfaceCollectors[name] = reader
				}
			}
			reader.update(stats)
		}
	}

	// TODO regularly check, if one of the observed interfaces does not exist anymore
	// Not every updated includes all interfaces.
	return nil
}

func (parent *Collector) parseRowUpdate(row libovsdb.Row) (name string, stats map[string]float64, err error) {
	defer func() {
		// Allow panics for less explicit type checks
		if rec := recover(); rec != nil {
			err = fmt.Errorf("Parsing OVSDB row updated failed: %v", rec)
		}
	}()

	if nameObj, ok := row.Fields["name"]; !ok {
		err = errors.New("Row update did not include 'name' field")
		return
	} else {
		name = nameObj.(string)
	}
	if statsObj, ok := row.Fields["statistics"]; !ok {
		err = errors.New("Row update did not include 'statistics' field")
	} else {
		statMap := statsObj.(libovsdb.OvsMap)
		stats = make(map[string]float64)
		for keyObj, valObj := range statMap.GoMap {
			stats[keyObj.(string)] = valObj.(float64)
		}
	}
	return
}
