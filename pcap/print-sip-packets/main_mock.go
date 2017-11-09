// +build nopcap

package main

import log "github.com/sirupsen/logrus"

func main() {
	log.Fatalln("This package cannot be built with the 'nopcap' build tag")
}
