package ipam

import (
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"tiny_cni/internal/log"

	"github.com/containernetworking/cni/pkg/types"
	cip "github.com/containernetworking/plugins/pkg/ip"
)

type Record struct {
	Cidr        *types.IPNet
	allocRecord map[string]bool
}

func (r *Record) getInnerIPNet() *net.IPNet {
	return (*net.IPNet)(r.Cidr)
}
func (r *Record) Alloced(ip *net.IP) bool {
	cidr := r.getInnerIPNet()
	if !cidr.Contains(*ip) {
		return false
	}
	if val, ok := r.allocRecord[ip.String()]; ok {
		if val {
			return true
		}
		delete(r.allocRecord, ip.String())
	}
	return false
}
func (r *Record) getAvailableLen() int {
	ones, bits := r.Cidr.Mask.Size()
	return bits - ones
}
func (r *Record) Gateway() *net.IPNet {
	if r.getAvailableLen() < 2 {
		return nil
	}
	return &net.IPNet{IP: cip.NextIP(r.Cidr.IP), Mask: r.Cidr.Mask}
}
func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func intToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes())
}
func (r *Record) Alloc() net.IP {
	size := r.getAvailableLen()
	if size < 2 {
		log.Log.Error("too small subnet")
		return nil
	}
	if size > 64 {
		log.Log.Warn("too big subnet")
		size = 64
	}
	max := (uint64(1) << size) - 3
	if len(r.allocRecord) >= max {
		log.Log.Error("subnet have no available ip addr")
		return nil
	}
	for {
		idx := rand.Uint64() % max
		ipNum := ipToInt(r.Gateway().IP)
		ip := intToIP(ipNum.Add(ipNum, idx))
		if !r.Alloced(&ip) {
			r.allocRecord[ip.String()] = true
			return ip
		}
	}
}
func (r *Record) Release(ip *net.IP) error {
	if r.Alloced(ip) == false {
		return fmt.Errorf("ip:%s is not alloced", ip.String())
	}
	delete(r.allocRecord, ip.String())
	return nil
}
