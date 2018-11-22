// +build !nolibvirt

package libvirt

import (
	"errors"
	"fmt"

	lib "github.com/libvirt/libvirt-go"
	log "github.com/sirupsen/logrus"
)

const (
	NO_FLAGS             = 0
	FETCH_DOMAINS_FLAGS  = lib.CONNECT_LIST_DOMAINS_ACTIVE | lib.CONNECT_LIST_DOMAINS_RUNNING
	MAX_NUM_MEMORY_STATS = 8
)

func NewDriver() Driver {
	return new(DriverImpl)
}

type DriverImpl struct {
	uri  string
	conn *lib.Connect
}

func (d *DriverImpl) Connect(uri string) error {
	d.uri = uri
	return nil
}

func (d *DriverImpl) ListDomains() ([]Domain, error) {
	conn, err := d.connection()
	if err != nil {
		return nil, err
	}
	virDomains, err := conn.ListAllDomains(FETCH_DOMAINS_FLAGS)
	if err != nil {
		return nil, err
	}
	domains := make([]Domain, len(virDomains))
	for i, domain := range virDomains {
		domains[i] = &DomainImpl{domain}
	}
	return domains, nil
}

func (d *DriverImpl) connection() (*lib.Connect, error) {
	conn := d.conn
	if conn != nil {
		if alive, err := conn.IsAlive(); err != nil || !alive {
			log.Warnln("Libvirt alive connection check failed:", err)
			if err := d.Close(); err != nil {
				return nil, err
			}
			conn = nil
		}
	}
	if conn == nil {
		if d.uri == "" {
			return nil, errors.New("Drier.Connect() has not yet been called.")
		}
		var err error
		conn, err = lib.NewConnect(d.uri)
		if err != nil {
			return nil, err
		}
		d.conn = conn
	}
	return conn, nil
}

func (d *DriverImpl) Close() (err error) {
	if d.conn != nil {
		_, err = d.conn.Close()
		d.conn = nil
	}
	return
}

type DomainImpl struct {
	domain lib.Domain
}

func (d *DomainImpl) GetName() (string, error) {
	return d.domain.GetName()
}

func (d *DomainImpl) CpuStats() (VirDomainCpuStats, error) {
	var res VirDomainCpuStats
	statSlice, err := d.domain.GetCPUStats(-1, 1, NO_FLAGS)
	if err == nil && len(statSlice) != 1 {
		err = fmt.Errorf("Libvirt returned %v CPU stats instead of 1: %v", len(statSlice), statSlice)
	}
	if err != nil {
		return res, err
	}
	stats := statSlice[0]
	res.CpuTime = stats.CpuTime
	res.SystemTime = stats.SystemTime
	res.UserTime = stats.UserTime
	res.VcpuTime = stats.VcpuTime
	return res, nil
}

func (d *DomainImpl) BlockStats(dev string) (VirDomainBlockStats, error) {
	stats, err := d.domain.BlockStats(dev)
	return VirDomainBlockStats{
		RdReq:   stats.RdReq,
		WrReq:   stats.WrReq,
		RdBytes: stats.RdBytes,
		WrBytes: stats.WrBytes,
	}, err
}

func (d *DomainImpl) BlockInfo(dev string) (VirDomainBlockInfo, error) {
	stats, err := d.domain.GetBlockInfo(dev, NO_FLAGS)
	return VirDomainBlockInfo{
		Allocation: stats.Allocation,
		Capacity:   stats.Capacity,
		Physical:   stats.Physical,
	}, err
}

func (d *DomainImpl) MemoryStats() (VirDomainMemoryStat, error) {
	var res VirDomainMemoryStat
	stats, err := d.domain.MemoryStats(MAX_NUM_MEMORY_STATS, NO_FLAGS)
	if err != nil {
		return res, err
	}
	for _, stat := range stats {
		switch stat.Tag {
		case int32(lib.DOMAIN_MEMORY_STAT_UNUSED):
			res.Unused = stat.Val
		case int32(lib.DOMAIN_MEMORY_STAT_AVAILABLE):
			res.Available = stat.Val
		}
	}
	return res, nil
}

func (d *DomainImpl) InterfaceStats(interfaceName string) (VirDomainInterfaceStats, error) {
	stats, err := d.domain.InterfaceStats(interfaceName)
	return VirDomainInterfaceStats{
		RxBytes:   stats.RxBytes,
		RxPackets: stats.RxPackets,
		RxErrs:    stats.RxErrs,
		RxDrop:    stats.RxDrop,
		TxBytes:   stats.TxBytes,
		TxPackets: stats.TxPackets,
		TxErrs:    stats.TxErrs,
		TxDrop:    stats.TxDrop,
	}, err
}

func (d *DomainImpl) GetXML() (string, error) {
	return d.domain.GetXMLDesc(NO_FLAGS)
}

func (d *DomainImpl) GetInfo() (res DomainInfo, err error) {
	var info *lib.DomainInfo
	info, err = d.domain.GetInfo()
	if err != nil {
		return
	}
	res.CpuTime = info.CpuTime
	res.MaxMem = info.MaxMem
	res.Mem = info.Memory
	return
}
