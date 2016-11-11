package libvirt

import (
	"fmt"
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	lib "github.com/rgbkrk/libvirt-go"
)

const (
	NO_FLAGS            = 0
	FETCH_DOMAINS_FLAGS = lib.VIR_CONNECT_LIST_DOMAINS_ACTIVE | lib.VIR_CONNECT_LIST_DOMAINS_RUNNING

	tolerated_vm_update_errors = 3
	DomainReparseInterval      = 1 * time.Minute
)

/*
	// TODO info about the node/hypervisor?

	// State
	v.GetState()
	v.IsActive()

	// Info
	v.GetID()
	v.GetMetadata()
	v.GetName()
	v.GetUUID()
	v.GetUUIDString()
	v.GetAutostart()
*/

func RegisterLibvirtCollector(connectUri string, factory *collector.ValueRingFactory) {
	collector.RegisterCollector(&LibvirtCollector{ConnectUri: connectUri, Factory: factory})
}

func LibvirtSsh(host string, keyfile string) string {
	if keyfile != "" {
		keyfile = "&keyfile=" + keyfile
	}
	return fmt.Sprintf("qemu+ssh://%s/system?no_verify=1%s", host, keyfile)
}

func LibvirtLocal() string {
	return "qemu:///system"
}
