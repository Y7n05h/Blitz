package ipam

import (
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"time"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/ipnet"

	cip "github.com/containernetworking/plugins/pkg/ip"
	"github.com/vishalkuo/bimap"
)

var _ json.Unmarshaler = (*Record)(nil)
var _ json.Marshaler = (*Record)(nil)

func init() {
	rand.Seed(time.Now().Unix() + int64(os.Getpid()))
}

type Record struct {
	//Subnet is available in this node
	Subnet *ipnet.IPNet
	//Gateway is the gateway of devices in this node
	//Sometimes, Gateway may not equal to Subnet
	Gateway *ipnet.IPNet
	//	IP  -> ID
	AllocRecord *bimap.BiMap[string, string]
}

func (r Record) MarshalJSON() ([]byte, error) {
	log.Log.Debug("Marshal Record Begin")
	data, err := json.Marshal(&struct {
		Subnet      ipnet.IPNet
		Gateway     ipnet.IPNet
		AllocRecord map[string]string
	}{
		Subnet:      *r.Subnet,
		Gateway:     *r.Gateway,
		AllocRecord: r.AllocRecord.GetForwardMap(),
	})
	if err != nil {
		log.Log.Fatal("Encode failed")
	}
	log.Log.Debug("Marshal Record Finished")
	return data, nil
}
func (r *Record) UnmarshalJSON(data []byte) error {
	log.Log.Debug("Unmarshal Record Begin")
	record := &struct {
		Subnet      ipnet.IPNet
		Gateway     ipnet.IPNet
		AllocRecord map[string]string
	}{}
	if err := json.Unmarshal(data, record); err != nil {
		return err
	}
	r.Subnet = &record.Subnet
	r.Gateway = &record.Gateway
	log.Log.Debugf("Get NodeCIDR: %s", r.Subnet.String())
	if record.AllocRecord != nil {
		r.AllocRecord = bimap.NewBiMapFromMap[string, string](record.AllocRecord)
	} else {
		r.AllocRecord = bimap.NewBiMap[string, string]()
	}
	log.Log.Debug("Unmarshal Record Finished")
	return nil
}
func New(subnet *ipnet.IPNet) *Record {
	return &Record{Subnet: subnet, Gateway: ipnet.FromIPAndMask(cip.NextIP(subnet.IP), subnet.Mask), AllocRecord: bimap.NewBiMap[string, string]()}
}
func (r *Record) Alloced(ip *net.IP) bool {
	log.Log.Debugf("Record %#v", r.AllocRecord)
	cidr := r.Subnet.ToNetIPNet()
	if !cidr.Contains(*ip) {
		return false
	}
	_, ok := r.AllocRecord.Get(ip.String())
	return ok
}
func (r *Record) getAvailableLen() int {
	ones, bits := r.Subnet.Mask.Size()
	return bits - ones
}
func (r *Record) GetGateway() *ipnet.IPNet {
	return r.Gateway
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
func (r *Record) Alloc(id string) (*ipnet.IPNet, error) {
	log.Log.Debugf("Record %#v", r.AllocRecord)
	subnet, ok := r.GetIPByID(id)
	if ok {
		return subnet, nil
	}

	log.Log.Debugf("Alloc Subnet %s", r.Subnet.String())
	size := r.getAvailableLen()
	if size < 2 {
		return nil, fmt.Errorf("too small Subnet")
	}
	if size > 64 {
		log.Log.Warn("too big Subnet")
		size = 64
	}
	max := (uint64(1) << size) - 3
	log.Log.Debugf("Max Size %d", max)
	if uint64(r.AllocRecord.Size()) >= max {
		return nil, fmt.Errorf("Subnet have no available ip addr")
	}
	idx := big.NewInt(0)
	for {
		idx.SetUint64(rand.Uint64() % max)
		log.Log.Debugf("Rand Number:%v", idx)
		ipNum := ipToInt(r.Subnet.IP)
		ip := intToIP(ipNum.Add(ipNum, idx))
		if !r.Alloced(&ip) {
			r.AllocRecord.Insert(ip.String(), id)
			log.Log.Debugf("Alloc IP %s", ip.String())
			return &ipnet.IPNet{IP: ip, Mask: r.Mask()}, nil
		}
	}
}
func (r *Record) Release(id string) error {
	log.Log.Debug("Release id")
	r.AllocRecord.DeleteInverse(id)
	log.Log.Debug("Release id Done")
	return nil
}

/*
GetIPByID never return nil,true
The First return value is nil iff the second value is false
*/
func (r *Record) GetIPByID(id string) (*ipnet.IPNet, bool) {
	ipString, ok := r.AllocRecord.GetInverse(id)
	if !ok {
		return nil, false
	}
	ip := net.ParseIP(ipString)
	if ip == nil {
		log.Log.Errorf("Parse Error:%s", ipString)
		return nil, false
	}
	return &ipnet.IPNet{IP: ip, Mask: r.Mask()}, true
}
func (r *Record) Mask() net.IPMask {
	return r.Gateway.Mask
}
