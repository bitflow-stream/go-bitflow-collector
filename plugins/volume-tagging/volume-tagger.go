package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/antongulenko/golib"
	"github.com/bitflow-stream/go-bitflow-collector/libvirt"
	"github.com/bitflow-stream/go-bitflow/bitflow"
	"github.com/bitflow-stream/go-bitflow/script/reg"
	log "github.com/sirupsen/logrus"
)

func RegisterLibvirtVolumeTagger(name string, b reg.ProcessorRegistry) {
	_ = b.RegisterStep(name, func(p *bitflow.SamplePipeline, params map[string]interface{}) error {
		p.Add(NewLibvirtVolumeTagger(
			params["uri"].(string),
			libvirt.NewDriver(),
			params["volumeKey"].(string),
			params["libvirtInstanceKey"].(string)))
		return nil
	}, "Append volume IDs to libvirt VM samples.").
		Optional("uri", reg.String(), libvirt.LocalUri).
		Optional("volumeKey", reg.String(), "volumes").
		Optional("libvirtInstanceKey", reg.String(), "vm")
}

func NewLibvirtVolumeTagger(uri string, driver libvirt.Driver, volumeKey string, libvirtInstanceKey string) *LibvirtVolumeTagger {
	return &LibvirtVolumeTagger{
		connectUri:         uri,
		driver:             driver,
		volumeKey:          volumeKey,
		libvirtInstanceKey: libvirtInstanceKey,
	}
}

type LibvirtVolumeTagger struct {
	bitflow.NoopProcessor

	connectUri string
	driver     libvirt.Driver

	volumeKey          string
	libvirtInstanceKey string

	domains             map[string]libvirt.Domain
	instance2VolumeInfo map[string][]libvirt.VolumeInfo

	updateDelay time.Duration
	lastUpdate  time.Time
}

func (l *LibvirtVolumeTagger) Init() error {
	return l.updateAllVolumeInfos()
}

func (l *LibvirtVolumeTagger) Start(wg *sync.WaitGroup) golib.StopChan {
	if err := l.Init(); err != nil {
		log.Warn("Error while initially updating libvirt domains.")
	}
	return l.NoopProcessor.Start(wg)
}

func (l *LibvirtVolumeTagger) String() string {
	return fmt.Sprintf("Libvirt volume tagger connected to libvirt via %v", l.connectUri)
}

func (l *LibvirtVolumeTagger) fetchDomains() error {
	if err := l.driver.Connect(l.connectUri); err != nil {
		return err
	}
	domains, err := l.driver.ListDomains()
	if err != nil {
		return err
	}
	for _, domain := range domains {
		if name, err := domain.GetName(); err != nil {
			return err
		} else {
			l.domains[name] = domain
		}
	}
	return nil
}

func (l *LibvirtVolumeTagger) updateVolumeInfos(libvirtInstance string) error {
	if domain, ok := l.domains[libvirtInstance]; ok {
		if volumeInfo, err := domain.GetVolumeInfo(); err == nil {
			l.instance2VolumeInfo[libvirtInstance] = volumeInfo
		} else {
			return err
		}
	}
	return nil
}

func (l *LibvirtVolumeTagger) updateAllVolumeInfos() error {
	// Clear old entries
	l.domains = make(map[string]libvirt.Domain)
	l.instance2VolumeInfo = make(map[string][]libvirt.VolumeInfo)

	if err := l.fetchDomains(); err != nil {
		return err
	}
	var errors golib.MultiError
	for instance := range l.domains {
		errors.Add(l.updateVolumeInfos(instance))
	}
	l.lastUpdate = time.Now()
	return errors.NilOrError()
}

func (l *LibvirtVolumeTagger) Sample(sample *bitflow.Sample, header *bitflow.Header) error {
	// Check sample tag for vm
	libvirtInstance := sample.Tag(l.libvirtInstanceKey)
	if libvirtInstance != "" {
		if l.lastUpdate.After(l.lastUpdate.Add(l.updateDelay)) { // Reloading buffered information
			if err := l.updateAllVolumeInfos(); err != nil {
				log.Warn("Error while loading volume information: ", err)
			}
		}
		if volumeInfo, ok := l.instance2VolumeInfo[libvirtInstance]; ok { // There are buffered volume information
			var volumeIds []string
			for _, info := range volumeInfo { // Make a list out of the IDs
				if info.Image != "" { // Only consider non-empty entries
					volumeIds = append(volumeIds, info.Image)
				}
			}
			if len(volumeIds) > 0 {
				volumeIdsStr := strings.Join(volumeIds, "|")
				sample.SetTag(l.volumeKey, volumeIdsStr) // String list representation as tag
			}
		}
	}
	return l.NoopProcessor.Sample(sample, header)
}

func (l *LibvirtVolumeTagger) Close() {
	if err := l.driver.Close(); err != nil {
		log.Errorf("Error closing libvirt connection to %v: %v", l.connectUri, err)
	}
	l.NoopProcessor.Close()
}
