package ipnet

import (
	"encoding/json"
	"net"
	"strings"

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
func ParseCIDRs(s string) []*IPNet {
	cidrStrings := strings.Split(s, ",")
	result := make([]*IPNet, 0)
	for _, cidrString := range cidrStrings {
		if cidr, err := ParseCIDR(cidrString); err == nil && cidr != nil {
			result = append(result, cidr)
		}
	}
	return result
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

	*n = *tmp
	return nil
}
func (n *IPNet) ToNetIPNet() *net.IPNet {
	return (*net.IPNet)(n)
}
func (n *IPNet) ToTypesIPNet() *types.IPNet {
	return (*types.IPNet)(n)
}
func (n *IPNet) Equal(s *IPNet) bool {
	return (n == s) || (n != nil && s != nil && n.IP.Equal(s.IP) && len(n.Mask) == len(s.Mask) && string(s.Mask) == string(s.Mask))
}
func (n *IPNet) String() string {
	return n.ToNetIPNet().String()
}
func (n *IPNet) IsIPv4() bool {
	return n.IP.To4() != nil
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
