// +build nolibvirt

package libvirt

import "errors"

var _ Driver = new(MockDriver)
var _ Domain = new(MockDomain)

func NewDriver() Driver {
	return new(MockDriver)
}

type MockDriver struct {
	uri         string
	InjectedErr error
}

func (d *MockDriver) Connect(uri string) error {
	if d.InjectedErr != nil {
		return d.InjectedErr
	}
	d.uri = uri
	return nil
}

func (d *MockDriver) ListDomains() ([]Domain, error) {
	if err := d.err(); err != nil {
		return nil, err
	}
	return nil, nil
}

func (d *MockDriver) Close() error {
	d.uri = ""
	return d.InjectedErr
}

func (d *MockDriver) err() error {
	if d.uri == "" {
		return errors.New("MockDomain: not connected")
	}
	return d.InjectedErr
}

type MockDomain struct {
	driver *MockDriver
}

func (d *MockDomain) err() error {
	return d.driver.err()
}

func (d *MockDomain) GetXML() (string, error) {
	return "", d.err()
}

func (d *MockDomain) GetInfo() (DomainInfo, error) {
	return DomainInfo{}, d.err()
}

func (d *MockDomain) GetName() (string, error) {
	return "", d.err()
}

func (d *MockDomain) CpuStats() (VirDomainCpuStats, error) {
	return VirDomainCpuStats{}, d.err()
}

func (d *MockDomain) BlockStats(dev string) (VirDomainBlockStats, error) {
	return VirDomainBlockStats{}, d.err()
}

func (d *MockDomain) BlockInfo(dev string) (VirDomainBlockInfo, error) {
	return VirDomainBlockInfo{}, d.err()
}

func (d *MockDomain) MemoryStats() (VirDomainMemoryStat, error) {
	return VirDomainMemoryStat{}, d.err()
}

func (d *MockDomain) InterfaceStats(interfaceName string) (VirDomainInterfaceStats, error) {
	return VirDomainInterfaceStats{}, d.err()
}
