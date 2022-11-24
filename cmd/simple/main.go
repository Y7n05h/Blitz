package main

import (
	"flag"
	"tiny_cni/internal/log"
)

type Flags struct {
	nwCfgGen    bool
	clusterCIDR string
}

var FlagsValue Flags

func init() {
	flag.BoolVar(&FlagsValue.nwCfgGen, "NetworkCfgGen", false, "Generator Network Cfg")
	flag.StringVar(&FlagsValue.clusterCIDR, "ClusterCIDR", "", "")
}
func main() {
	flag.Parse()
	log.Log.Debugf("flags:%#v", FlagsValue)
}
