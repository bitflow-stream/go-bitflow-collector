// +build nopcap

package main

import log "github.com/Sirupsen/logrus"

func main() {
	log.Fatalln("This package cannot be built with the 'nopcap' build tag")
}
