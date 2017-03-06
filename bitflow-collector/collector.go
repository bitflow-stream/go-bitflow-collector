package main

import (
	"flag"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antongulenko/go-bitflow-collector"
	"github.com/antongulenko/go-bitflow-collector/libvirt"
	"github.com/antongulenko/go-bitflow-collector/mock"
	"github.com/antongulenko/go-bitflow-collector/ovsdb"
	"github.com/antongulenko/go-bitflow-collector/pcap"
	"github.com/antongulenko/go-bitflow-collector/psutil"
	"github.com/antongulenko/golib"
)

var (
	collect_local_interval = 500 * time.Millisecond
	sink_interval          = 500 * time.Millisecond

	all_metrics           = false
	include_basic_metrics = false
	user_include_metrics  golib.StringSlice
	user_exclude_metrics  golib.StringSlice

	proc_collectors  golib.KeyValueStringSlice
	proc_show_errors = false
	proc_update_pids = 1500 * time.Millisecond

	libvirt_uri = libvirt.LocalUri // libvirt.SshUri("host", "keyfile")
	ovsdb_host  = ""

	pcap_nics = ""

	updateFrequencies = map[*regexp.Regexp]time.Duration{
		regexp.MustCompile("^psutil/pids$"):       1500 * time.Millisecond, // Changed processes
		regexp.MustCompile("^psutil/disk-usage$"): 5 * time.Second,         // Changed local partitions
		regexp.MustCompile("^libvirt$"):           10 * time.Second,        // New VMs
		regexp.MustCompile("^libvirt/[^/]+$"):     30 * time.Second,        // Changed VM configuration
	}

	ringFactory = collector.ValueRingFactory{
		// This is an important package-wide constant: time-window for all aggregated values
		Interval: 1000 * time.Millisecond,
	}
)

const (
	FailedCollectorCheckInterval   = 5 * time.Second
	FilteredCollectorCheckInterval = 3 * time.Second
)

var (
	includeMetricsRegexes []*regexp.Regexp
	excludeMetricsRegexes = []*regexp.Regexp{
		regexp.MustCompile("^mock/.$"),
		regexp.MustCompile("^net-proto/(UdpLite|IcmpMsg)"),                         // Some extended protocol-metrics
		regexp.MustCompile("^disk-io/[^all]"),                                      // Disk IO for specific partitions/disks
		regexp.MustCompile("^disk-usage/[^all]"),                                   // Disk usage for specific partitions
		regexp.MustCompile("^net-proto/tcp/(MaxConn|RtpAlgorithm|RtpMin|RtoMax)$"), // Some irrelevant TCP/IP settings
		regexp.MustCompile("^net-proto/ip/(DefaultTTL|Forwarding)$"),
	}
	includeBasicMetricsRegexes = []*regexp.Regexp{
		regexp.MustCompile("^(cpu|mem/percent)$"),
		regexp.MustCompile("^disk-io/all/(io|ioTime|ioBytes)$"),
		regexp.MustCompile("^net-io/(bytes|packets|dropped|errors)$"),
		regexp.MustCompile("^proc/.+/(cpu|mem/rss|disk/(io|ioBytes)|net-io/(bytes|packets|dropped|errors))$"),
	}
)

func init() {
	flag.StringVar(&libvirt_uri, "libvirt", libvirt_uri, "Libvirt connection uri (default is local system)")
	flag.StringVar(&ovsdb_host, "ovsdb", ovsdb_host, "OVSDB host to connect to. Empty for localhost. Port is "+strconv.Itoa(ovsdb.DefaultOvsdbPort))
	flag.BoolVar(&all_metrics, "a", all_metrics, "Disable built-in filters on available metrics")
	flag.Var(&user_exclude_metrics, "exclude", "Metrics to exclude (substring match)")
	flag.Var(&user_include_metrics, "include", "Metrics to include exclusively (substring match)")
	flag.BoolVar(&include_basic_metrics, "basic", include_basic_metrics, "Include only a certain basic subset of metrics")

	flag.Var(&proc_collectors, "proc", "'key=regex' Processes to collect metrics for (regex match on entire command line)")
	flag.BoolVar(&proc_show_errors, "proc-show-errors", proc_show_errors, "Verbose: show errors encountered while getting process metrics")
	flag.DurationVar(&proc_update_pids, "proc-interval", proc_update_pids, "Interval for updating list of observed pids")

	flag.DurationVar(&collect_local_interval, "ci", collect_local_interval, "Interval for collecting local samples")
	flag.DurationVar(&sink_interval, "si", sink_interval, "Interval for sinking (sending/printing/...) data when collecting local samples")

	flag.StringVar(&pcap_nics, "nics", pcap_nics, "Comma-separated list of NICs to capture packets from for PCAP-based"+
		"monitoring of process network IO (/proc/.../net-pcap/...). Defaults to all physical NICs.")
}

func configurePcap() {
	if pcap_nics == "" {
		allNics, err := pcap.PhysicalInterfaces()
		if err != nil {
			golib.Fatalln("Failed to enumerate physical NICs:", err)
		}
		psutil.PcapNics = allNics
	} else {
		psutil.PcapNics = strings.Split(pcap_nics, ",")
	}
}

func createCollectorSource() *collector.CollectorSource {
	configurePcap()
	ringFactory.Length = int(ringFactory.Interval/collect_local_interval) * 10 // Make sure enough samples can be buffered
	var cols []collector.Collector

	cols = append(cols, mock.NewMockCollector(&ringFactory))
	psutilRoot := psutil.NewPsutilRootCollector(&ringFactory)
	cols = append(cols, psutilRoot)
	cols = append(cols, libvirt.NewLibvirtCollector(libvirt_uri, libvirt.NewDriver(), &ringFactory))
	cols = append(cols, ovsdb.NewOvsdbCollector(ovsdb_host, &ringFactory))

	if len(proc_collectors.Keys) > 0 {
		psutil.PidUpdateInterval = proc_update_pids
		regexes := make(map[string][]*regexp.Regexp)
		for key, value := range proc_collectors.Map() {
			regex, err := regexp.Compile(value)
			golib.Checkerr(err)
			regexes[key] = append(regexes[key], regex)
		}
		for key, list := range regexes {
			cols = append(cols, psutilRoot.NewProcessCollector(list, key, proc_show_errors))
		}
	}
	if all_metrics {
		excludeMetricsRegexes = nil
	}
	if include_basic_metrics {
		includeMetricsRegexes = append(includeMetricsRegexes, includeBasicMetricsRegexes...)
	}
	for _, exclude := range user_exclude_metrics {
		regex, err := regexp.Compile(exclude)
		if err != nil {
			golib.Checkerr(fmt.Errorf("Error compiling exclude regex: %v", err))
		}
		excludeMetricsRegexes = append(excludeMetricsRegexes, regex)
	}
	for _, include := range user_include_metrics {
		regex, err := regexp.Compile(include)
		if err != nil {
			golib.Checkerr(fmt.Errorf("Error compiling include regex: %v", err))
		}
		includeMetricsRegexes = append(includeMetricsRegexes, regex)
	}

	return &collector.CollectorSource{
		RootCollectors:                 cols,
		UpdateFrequencies:              updateFrequencies,
		CollectInterval:                collect_local_interval,
		SinkInterval:                   sink_interval,
		ExcludeMetrics:                 excludeMetricsRegexes,
		IncludeMetrics:                 includeMetricsRegexes,
		FailedCollectorCheckInterval:   FailedCollectorCheckInterval,
		FilteredCollectorCheckInterval: FilteredCollectorCheckInterval,
	}
}
