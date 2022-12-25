package ipam

import (
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"os"
	"time"

	cip "github.com/containernetworking/plugins/pkg/ip"
	"github.com/vishalkuo/bimap"
)

var _ json.Unmarshaler = (*Ipam)(nil)
var _ json.Marshaler = (*Ipam)(nil)

func init() {
	rand.Seed(time.Now().Unix() + int64(os.Getpid()))
}

type Ipam struct {
	//Subnet is available in this node
	Subnet *ipnet.IPNet
	//gateway is the gateway of devices in this node
	//Sometimes, gateway may not equal to Subnet
	gateway      *ipnet.IPNet
	validIPCount uint64
	//	IP  -> ID
	AllocRecord *bimap.BiMap[string, string]
}

func validIpCount(subnet *ipnet.IPNet) uint64 {
	ones, bits := subnet.Mask.Size()
	size := bits - ones
	if size < 2 {
		log.Log.Warn("too small Subnet")
		return 0
	}
	if size > 64 {
		log.Log.Warn("too big Subnet")
		size = 64
	}
	// 减去 2 个不可用于 Pod 的地址（主机号全为 0 或全为 1）
	//和 1 个 Blitz 的保留地址（主机号为 1）
	result := (uint64(1) << size) - 3
	log.Log.Debugf("Max Size %d", result)
	return result
}
func (r Ipam) MarshalJSON() ([]byte, error) {
	log.Log.Debug("Marshal Ipam Begin")
	data, err := json.Marshal(&struct {
		Subnet      ipnet.IPNet
		AllocRecord map[string]string
	}{
		Subnet:      *r.Subnet,
		AllocRecord: r.AllocRecord.GetForwardMap(),
	})
	if err != nil {
		log.Log.Fatal("Encode failed")
	}
	log.Log.Debug("Marshal Ipam Finished")
	return data, nil
}
func (r *Ipam) UnmarshalJSON(data []byte) error {
	log.Log.Debug("Unmarshal Ipam Begin")
	record := &struct {
		Subnet      ipnet.IPNet
		AllocRecord map[string]string
	}{}
	if err := json.Unmarshal(data, record); err != nil {
		return err
	}
	r.Subnet = &record.Subnet
	r.gateway = ipnet.FromIPAndMask(cip.NextIP(record.Subnet.IP), record.Subnet.Mask)
	log.Log.Debugf("Get PodCIDR: %s", r.Subnet.String())
	if record.AllocRecord != nil {
		r.AllocRecord = bimap.NewBiMapFromMap[string, string](record.AllocRecord)
	} else {
		r.AllocRecord = bimap.NewBiMap[string, string]()
	}
	r.validIPCount = validIpCount(r.Subnet)
	log.Log.Debug("Unmarshal Ipam Finished")
	return nil
}
func New(subnet *ipnet.IPNet) *Ipam {
	return &Ipam{
		Subnet:       subnet,
		gateway:      ipnet.FromIPAndMask(cip.NextIP(subnet.IP), subnet.Mask),
		validIPCount: validIpCount(subnet),
		AllocRecord:  bimap.NewBiMap[string, string](),
	}
}
func (r *Ipam) Alloced(ip *net.IP) bool {
	log.Log.Debugf("Ipam %#v", r.AllocRecord)
	cidr := r.Subnet.ToNetIPNet()
	if !cidr.Contains(*ip) {
		return false
	}
	_, ok := r.AllocRecord.Get(ip.String())
	return ok
}
func (r *Ipam) getAvailableLen() int {
	ones, bits := r.Subnet.Mask.Size()
	return bits - ones
}
func (r *Ipam) GetGateway() *ipnet.IPNet {
	return r.gateway
}
func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func intToIP(i *big.Int) net.IP {
	return i.Bytes()
}
func (r *Ipam) saveRecord(ip net.IP, id string) *ipnet.IPNet {
	log.Log.Debugf("Alloc IP %s", ip.String())
	r.AllocRecord.Insert(ip.String(), id)
	return &ipnet.IPNet{IP: ip, Mask: r.Mask()}
}
func (r *Ipam) Alloc(id string) (*ipnet.IPNet, error) {
	log.Log.Debugf("Ipam %#v", r.AllocRecord)
	subnet, ok := r.GetIPByID(id)
	if ok {
		return subnet, nil
	}

	max := r.validIPCount
	usedIPAddr := uint64(r.AllocRecord.Size())
	if usedIPAddr >= max {
		return nil, fmt.Errorf("subnet have no available ip addr")
	}
	idx := big.NewInt(0)
	if (usedIPAddr << 1) <= max {
		//剩余的IP地址充足，通过遍历来寻找可用的 IP 地址
		for {
			idx.SetUint64((rand.Uint64() % max) + 2)
			log.Log.Debugf("Rand Number:%v", idx)
			ipNum := ipToInt(r.Subnet.IP)
			ip := intToIP(ipNum.Add(ipNum, idx))
			if !r.Alloced(&ip) {
				return r.saveRecord(ip, id), nil
			}
		}
	}
	for n := uint64(0); n < max; n++ {
		idx.SetUint64(n + 2)
		log.Log.Debugf("Rand Number:%v", idx)
		ipNum := ipToInt(r.Subnet.IP)
		ip := intToIP(ipNum.Add(ipNum, idx))
		if !r.Alloced(&ip) {
			return r.saveRecord(ip, id), nil
		}
	}
	return nil, fmt.Errorf("alloc IP Failed")
}
func (r *Ipam) Release(id string) error {
	log.Log.Debug("Release id")
	r.AllocRecord.DeleteInverse(id)
	log.Log.Debug("Release id Done")
	return nil
}

/*
GetIPByID never return nil,true
The First return value is nil iff the second value is false
*/
func (r *Ipam) GetIPByID(id string) (*ipnet.IPNet, bool) {
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
func (r *Ipam) Mask() net.IPMask {
	return r.Subnet.Mask
}
