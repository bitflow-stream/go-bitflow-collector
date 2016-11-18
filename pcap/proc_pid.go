package pcap

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

const (
	procFdDir      = "/proc/%v/fd"
	socketLinkName = "socket:["
)

func (cons *Connections) FilterConnections(pids []int) ([]*Connection, error) {
	inodes := make(map[string]bool)
	for _, pid := range pids {
		if err := fillInodes(pid, inodes); err != nil {
			return nil, err
		}
	}

	cons.lock.RLock()
	cons.lock.RUnlock()
	var found []*Connection
	for _, con := range cons._map {
		if inodes[con.Inode] {
			found = append(found, con)
		}
	}
	return found, nil
}

func fillInodes(pid int, inodes map[string]bool) error {
	dir := fmt.Sprintf(procFdDir, pid)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, file := range files {
		link, err := os.Readlink(filepath.Join(dir, file.Name()))
		if err != nil {
			return err
		}
		if strings.HasPrefix(link, socketLinkName) {
			inode := link[len(socketLinkName) : len(link)-1]
			inodes[inode] = true
		}
	}
	return nil
}
