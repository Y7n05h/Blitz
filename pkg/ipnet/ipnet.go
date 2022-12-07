package ipnet

import (
	"encoding/json"
	"net"

	"github.com/containernetworking/cni/pkg/types"
)

type IPNet types.IPNet

var _ json.Unmarshaler = (*IPNet)(nil)
var _ json.Marshaler = (*IPNet)(nil)

func ParseCIDR(s string) (*IPNet, error) {
	ip, ipn, err := net.ParseCIDR(s)
	if err != nil {
		return nil, err
	}
	ipn.IP = ip
	return FromNetIPNet(ipn), nil
}

func (n IPNet) MarshalJSON() ([]byte, error) {
	return json.Marshal((*net.IPNet)(&n).String())
}

func (n *IPNet) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	tmp, err := ParseCIDR(s)
	if err != nil {
		return err
	}

	*n = IPNet(*tmp)
	return nil
}
func (n *IPNet) ToNetIPNet() *net.IPNet {
	return (*net.IPNet)(n)
}
func (n *IPNet) ToTypesIPNet() *types.IPNet {
	return (*types.IPNet)(n)
}
func (n *IPNet) Equal(s *IPNet) bool {
	if !n.IP.Equal(s.IP) {
		return false
	}
	if len(n.Mask) != len(s.Mask) {
		return false
	}
	return string(n.Mask) == string(s.Mask)
}
func (n *IPNet) String() string {
	return n.ToNetIPNet().String()
}
func FromTypesIPNet(ipnet *types.IPNet) *IPNet {
	return (*IPNet)(ipnet)
}
func FromNetIPNet(ipnet *net.IPNet) *IPNet {
	return (*IPNet)(ipnet)
}
func FromIPAndMask(ip net.IP, mask net.IPMask) *IPNet {
	return FromNetIPNet(&net.IPNet{
		IP:   ip,
		Mask: mask,
	})
}
