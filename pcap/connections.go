package pcap

import (
	"fmt"
	"sort"
	"sync"
)

type Connections struct {
	_map map[interface{}]*Connection
	lock sync.RWMutex
}

func NewConnections() *Connections {
	cons := &Connections{
		_map: make(map[interface{}]*Connection),
	}
	for _, typ := range AllConnectionTypes {
		con := &Connection{Type: typ}
		cons._map[con.Hash()] = con
	}
	return cons
}

func (cons *Connections) Sorted() []*Connection {
	cons.lock.RLock()
	defer cons.lock.RUnlock()
	slice := make(ConnectionSlice, 0, len(cons._map))
	for _, con := range cons._map {
		slice = append(slice, con)
	}
	sort.Sort(slice)
	return []*Connection(slice)
}

func (cons *Connections) LogPacket(info *Connection, size uint64) error {
	logged := cons.LogExistingConnection(info, size)
	if !logged {
		if err := cons.Update(); err != nil {
			return err
		}
		return cons.LogUnknownConnection(info.Type, size)
	}
	return nil
}

func (cons *Connections) LogExistingConnection(info *Connection, size uint64) (logged bool) {
	cons.lock.RLock()
	defer cons.lock.RUnlock()
	if con, ok := cons._map[info.Hash()]; ok {
		con.LogTxPacket(size)
		logged = true
	}
	if con, ok := cons._map[info.ReverseHash()]; ok {
		con.LogRxPacket(size)
		logged = true
	}
	return
}

func (cons *Connections) LogUnknownConnection(typ string, size uint64) error {
	defaultHash := HashConnection(typ, "", 0, "", 0)
	cons.lock.RLock()
	defer cons.lock.RUnlock()
	if con, ok := cons._map[defaultHash]; ok {
		con.LogRxPacket(size) // Arbitrarily choose "Rx", direction is not known
	} else {
		return fmt.Errorf("No default connection found for %v packet len %v.", typ, size)
	}
	return nil
}

func (cons *Connections) Update() error {
	allCons, err := ReadAllConnections()
	if err != nil {
		return err
	}
	cons.lock.Lock()
	defer cons.lock.Unlock()
	for _, con := range allCons {
		hash := con.Hash()
		if _, ok := cons._map[hash]; !ok {
			cons._map[hash] = con
		}
		reverseHash := con.Hash()
		if _, ok := cons._map[reverseHash]; !ok {
			cons._map[reverseHash] = con
		}
	}
	return nil
}

type ConnectionSlice []*Connection

func (s ConnectionSlice) Len() int {
	return len(s)
}

func (s ConnectionSlice) Less(i, j int) bool {
	a, b := s[i], s[j]
	if a.State == "" {
		if b.State == "" {
			return a.Type < b.Type
		} else {
			return true
		}
	}
	if b.State == "" {
		return false
	}

	switch {
	case a.Type < b.Type:
		return true
	case a.Type > b.Type:
		return false
	case a.State < b.State:
		return true
	case a.State > b.State:
		return false
	case a.Ip < b.Ip:
		return true
	case a.Ip > b.Ip:
		return false
	case a.Port < b.Port:
		return true
	case a.Port > b.Port:
		return false
	case a.Fip < b.Fip:
		return true
	case a.Fip > b.Fip:
		return false
	case a.Fport < b.Fport:
		return true
	case a.Fport > b.Fport:
		return false
	}
	return false
}

func (s ConnectionSlice) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
