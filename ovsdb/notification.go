package ovsdb

import "github.com/socketplane/libovsdb"

type ovsdbNotifier struct {
	col *Collector
}

func (n *ovsdbNotifier) Update(_ interface{}, tableUpdates libovsdb.TableUpdates) {
	// Note: Do not call n.col.client.Disconnect() from here (deadlock)
	if n.col.lastUpdateError != nil {
		return
	}
	if err := n.col.updateTables(true, tableUpdates.Updates); err != nil {
		n.col.lastUpdateError = err
	}
}
func (n *ovsdbNotifier) Locked([]interface{}) {
}
func (n *ovsdbNotifier) Stolen([]interface{}) {
}
func (n *ovsdbNotifier) Echo([]interface{}) {
}
func (n *ovsdbNotifier) Disconnected(client *libovsdb.OvsdbClient) {
	n.col.client = nil
}
