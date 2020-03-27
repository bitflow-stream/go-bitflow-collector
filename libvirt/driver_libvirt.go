// +build !nolibvirt

package libvirt

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	lib "github.com/libvirt/libvirt-go"
	log "github.com/sirupsen/logrus"
)

const (
	NoFlags                       = 0
	FetchDomainsFlags             = lib.CONNECT_LIST_DOMAINS_ACTIVE | lib.CONNECT_LIST_DOMAINS_RUNNING
	MaxNumMemoryStats             = 8
	domainQemuMonitorCommandFlags = lib.DOMAIN_QEMU_MONITOR_COMMAND_HMP
	qemuMonitorCommand            = "info block"
)

var volumeJsonRegex = regexp.MustCompile("json:{(.*)}")

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
	virDomains, err := conn.ListAllDomains(FetchDomainsFlags)
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
			if closeErr := d.Close(); closeErr != nil {
				return nil, closeErr
			}
			conn = nil
		}
	}
	if conn == nil {
		if d.uri == "" {
			return nil, errors.New("Driver.Connect() has not yet been called.")
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

func (d *DomainImpl) CpuStats() (res VirDomainCpuStats, err error) {
	var statSlice []lib.DomainCPUStats
	statSlice, err = d.domain.GetCPUStats(-1, 1, NoFlags)
	if err == nil && len(statSlice) != 1 {
		err = fmt.Errorf("Libvirt returned %v CPU stats instead of 1: %v", len(statSlice), statSlice)
	}
	if err == nil {
		stats := statSlice[0]
		res = VirDomainCpuStats{
			CpuTime:    stats.CpuTime,
			SystemTime: stats.SystemTime,
			UserTime:   stats.UserTime,
			VcpuTime:   stats.VcpuTime,
		}
	}
	return
}

func (d *DomainImpl) BlockStats(dev string) (res VirDomainBlockStats, err error) {
	var stats *lib.DomainBlockStats
	stats, err = d.domain.BlockStats(dev)
	if err == nil {
		res = VirDomainBlockStats{
			RdReq:   stats.RdReq,
			WrReq:   stats.WrReq,
			RdBytes: stats.RdBytes,
			WrBytes: stats.WrBytes,
		}
	}
	return
}

func (d *DomainImpl) BlockInfo(dev string) (res VirDomainBlockInfo, err error) {
	var stats *lib.DomainBlockInfo
	stats, err = d.domain.GetBlockInfo(dev, NoFlags)
	if err == nil {
		res = VirDomainBlockInfo{
			Allocation: stats.Allocation,
			Capacity:   stats.Capacity,
			Physical:   stats.Physical,
		}
	}
	return
}

func (d *DomainImpl) MemoryStats() (res VirDomainMemoryStat, err error) {
	var stats []lib.DomainMemoryStat
	stats, err = d.domain.MemoryStats(MaxNumMemoryStats, NoFlags)
	if err == nil {
		for _, stat := range stats {
			switch stat.Tag {
			case int32(lib.DOMAIN_MEMORY_STAT_UNUSED):
				res.Unused = stat.Val
			case int32(lib.DOMAIN_MEMORY_STAT_AVAILABLE):
				res.Available = stat.Val
			}
		}
	}
	return
}

func (d *DomainImpl) InterfaceStats(interfaceName string) (res VirDomainInterfaceStats, err error) {
	var stats *lib.DomainInterfaceStats
	stats, err = d.domain.InterfaceStats(interfaceName)
	if err == nil {
		res = VirDomainInterfaceStats{
			RxBytes:   stats.RxBytes,
			RxPackets: stats.RxPackets,
			RxErrs:    stats.RxErrs,
			RxDrop:    stats.RxDrop,
			TxBytes:   stats.TxBytes,volumeJsonRegex
			TxPackets: stats.TxPackets,
			TxErrs:    stats.TxErrs,
			TxDrop:    stats.TxDrop,
		}
	}
	return
}

func (d *DomainImpl) GetXML() (string, error) {
	return d.domain.GetXMLDesc(NoFlags)
}

func (d *DomainImpl) GetInfo() (res DomainInfo, err error) {
	var info *lib.DomainInfo
	info, err = d.domain.GetInfo()
	if err == nil {
		res.CpuTime = info.CpuTime
		res.MaxMem = info.MaxMem
		res.Mem = info.Memory
	}
	return
}

func (d *DomainImpl) GetVolumeInfo() (res []VolumeInfo, err error) {
	if volumeInfoStr, err := d.domain.QemuMonitorCommand(qemuMonitorCommand, domainQemuMonitorCommandFlags); err == nil {
		res = d.parseVolumeInfo(volumeInfoStr)
	}
	return
}

func (d *DomainImpl) parseVolumeInfo(volumeInfoStr string) []VolumeInfo {
	var result []VolumeInfo
	split := strings.Split(volumeInfoStr, "\n")
	for _, line := range split {
		if match := volumeJsonRegex.FindString(line); match != "" {
			var objmap1 map[string]json.RawMessage
			var objmap2 map[string]string
			b := []byte(match[5:]) // match without the "json:" prefix
			if err := json.Unmarshal(b, &objmap1); err == nil {
				if err := json.Unmarshal(objmap1["file"], &objmap2); err == nil {
					result = append(result, VolumeInfo{
						Pool:   objmap2["pool"],
						Image:  objmap2["image"],
						Driver: objmap2["driver"],
						User:   objmap2["user"],
					})
				}
			}
		}
	}
	return result
}
