# data-collection-agent
data-collection-agent is a Go (Golang) tool for collecting time-series data from various sources in high frequency intervals.
It uses the `github.com/antongulenko/data2go` library for sending `Sample`.
The `data-collection-agent` sub-package provides an executable with the same name.
The data collection and other configuration options can be configured through numerous command line flags.

Run `data-collection-agent --help` for a list of command line flags.

The main source of data is the `/proc` filesystem on the local Linux machine (although data collection should also work on other platforms in general).
Other implemented data sources include the remote API provided by `libvirt` and the `OVSDB` protocol offered by Open vSwitch.

## Installation:
* Install git and go (at least version **1.6**).
* Make sure `$GOPATH` is set to some existing directory.
* Execute the following command to make `go get` work with Gitlab:

```shell
git config --global "url.git@gitlab.tubit.tu-berlin.de:CIT-Huawei/data-collection-agent.git.insteadOf" "https://github.com/antongulenko/data-collection-agent"
```
* Get and install this tool:

```shell
go get github.com/antongulenko/data-collection-agent/data-collection-agent
```
* The binary executable `data-collection-agent` will be compiled to `$GOPATH/bin`.
 * Add that directory to your `$PATH`, or copy the executable to a different location.

