package ipam

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"tiny_cni/internal/log"

	"github.com/containernetworking/cni/pkg/types"
	cip "github.com/containernetworking/plugins/pkg/ip"
	"github.com/vishalkuo/bimap"
)

type Record struct {
	Cidr *types.IPNet
	//	IP  -> ID
	AllocRecord *bimap.BiMap[string, string]
}

func (r Record) Marshal() []byte {
	data, err := json.Marshal(&struct {
		Cidr        net.IPNet
		AllocRecord map[string]string
	}{
		Cidr:        *r.getInnerIPNet(),
		AllocRecord: r.AllocRecord.GetForwardMap(),
	})
	if err != nil {
		log.Log.Fatal("Encode failed")
	}
	return data
}
func (r *Record) Unmarshal(data []byte) error {
	log.Log.Debug("Unmarshal Record Begin")
	record := &struct {
		Cidr        net.IPNet
		AllocRecord map[string]string
	}{}
	if err := json.Unmarshal(data, record); err != nil {
		return err
	}
	r.Cidr = (*types.IPNet)(&record.Cidr)
	if record.AllocRecord == nil {
		r.AllocRecord = bimap.NewBiMapFromMap[string, string](record.AllocRecord)
	} else {
		r.AllocRecord = bimap.NewBiMap[string, string]()
	}
	log.Log.Debug("Unmarshal Record Finished")
	return nil
}
func New(subnet *net.IPNet) *Record {
	return &Record{Cidr: (*types.IPNet)(subnet), AllocRecord: bimap.NewBiMap[string, string]()}
}
func (r *Record) getInnerIPNet() *net.IPNet {
	return (*net.IPNet)(r.Cidr)
}
func (r *Record) Alloced(ip *net.IP) bool {
	cidr := r.getInnerIPNet()
	if !cidr.Contains(*ip) {
		return false
	}
	_, ok := r.AllocRecord.Get(ip.String())
	return ok
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
func (r *Record) Alloc(id string) (*net.IPNet, error) {
	ip, ok := r.GetIPByID(id)
	if ok {
		return &net.IPNet{IP: ip, Mask: r.Mask()}, nil
	}

	size := r.getAvailableLen()
	if size < 2 {
		return nil, fmt.Errorf("too small subnet")
	}
	if size > 64 {
		log.Log.Warn("too big subnet")
		size = 64
	}
	max := (uint64(1) << size) - 3
	if uint64(r.AllocRecord.Size()) >= max {
		return nil, fmt.Errorf("subnet have no available ip addr")
	}
	idx := big.NewInt(0)
	for {
		idx.SetUint64(rand.Uint64() % max)
		ipNum := ipToInt(r.Gateway().IP)
		ip := intToIP(ipNum.Add(ipNum, idx))
		if !r.Alloced(&ip) {
			r.AllocRecord.Insert(ip.String(), id)
			return &net.IPNet{
				IP:   ip,
				Mask: r.Cidr.Mask,
			}, nil
		}
	}
}
func (r *Record) Release(id string) error {
	log.Log.Debug("Release id")
	r.AllocRecord.DeleteInverse(id)
	log.Log.Debug("Release id Done")
	return nil
}
func (r *Record) GetIPByID(id string) (net.IP, bool) {
	ipString, ok := r.AllocRecord.GetInverse(id)
	if !ok {
		return nil, false
	}
	ip, _, err := net.ParseCIDR(ipString)
	if err != nil {
		log.Log.Error("Parse Error")
		return nil, false
	}
	return ip, true
}
func (r *Record) Mask() net.IPMask {
	return r.Cidr.Mask
}
