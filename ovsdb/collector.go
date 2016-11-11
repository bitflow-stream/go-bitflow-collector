package ovsdb

import (
	"fmt"
	"sync"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	"github.com/socketplane/libovsdb"
)

type OvsdbCollector struct {
	collector.AbstractCollector
	Host    string
	Port    int
	Factory *collector.ValueRingFactory

	client           *libovsdb.OvsdbClient
	lastUpdateError  error
	notifier         ovsdbNotifier
	interfaceReaders map[string]*ovsdbInterfaceReader
	readersLock      sync.Mutex
}

func (col *OvsdbCollector) Init() error {
	col.Reset(col)
	col.Close()
	col.notifier.col = col
	col.lastUpdateError = nil
	col.interfaceReaders = make(map[string]*ovsdbInterfaceReader)
	if err := col.update(false); err != nil {
		return err
	}

	col.Readers = make(map[string]collector.MetricReader)
	for _, reader := range col.interfaceReaders {
		reader.counters.Register(col.Readers, "ovsdb/"+reader.name)
	}
	return nil
}

func (col *OvsdbCollector) getReader(name string) *ovsdbInterfaceReader {
	if reader, ok := col.interfaceReaders[name]; ok {
		return reader
	}
	reader := &ovsdbInterfaceReader{
		col:      col,
		name:     name,
		counters: psutil.NewNetIoCounters(col.Factory),
	}
	col.interfaceReaders[name] = reader
	return reader
}

func (col *OvsdbCollector) Close() {
	if client := col.client; client != nil {
		client.Disconnect()
		col.client = nil
	}
}

func (col *OvsdbCollector) Update() (err error) {
	if err = col.update(true); err == nil {
		col.UpdateMetrics()
	}
	return
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

	// TODO periodically check, if all monitored interfaces still exist
	col.readersLock.Lock()
	defer col.readersLock.Unlock()
	for _, rowUpdate := range update.Rows {
		if name, stats, err := col.parseRowUpdate(&rowUpdate.New); err != nil {
			return err
		} else {
			if checkChange {
				if _, ok := col.interfaceReaders[name]; !ok {
					return collector.MetricsChanged
				}
			}
			col.getReader(name).update(stats)
		}
	}
	return nil
}

func (col *OvsdbCollector) parseRowUpdate(row *libovsdb.Row) (name string, stats map[string]float64, err error) {
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
