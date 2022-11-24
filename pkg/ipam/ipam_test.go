package ipam

import (
	"encoding/json"
	"net"
	"testing"
	"tiny_cni/internal/log"
)

func checkIPNetUnderSubnet(t *testing.T, ipNet net.IPNet, subnet net.IPNet) {
	if !subnet.Contains(ipNet.IP) {
		t.Fatalf("Alloc IP Error")
	}
	if ipNet.Mask.String() != subnet.Mask.String() {
		t.Fatalf("Alloc IP Mask Error")
	}
}
func equalIPNet(ipNet1 net.IPNet, ipNet2 net.IPNet) bool {
	return ipNet1.String() == ipNet2.String()

}
func TestRecord_Alloc(t *testing.T) {
	_, subnet, _ := net.ParseCIDR("192.168.1.1/24")
	record := New(subnet)
	id1 := "123123123"
	id2 := "312312312"
	ip1, err := record.Alloc(id1)
	if err != nil {
		t.Fatalf("Alloc Failed")
	}
	checkIPNetUnderSubnet(t, *ip1, *subnet)
	ip2, err := record.Alloc(id1)
	if err != nil {
		t.Fatalf("Alloc Failed")
	}
	if !equalIPNet(*ip1, *ip2) {
		t.Fatalf("Ip not Equal: ip1 %s ip2 %s", ip1.String(), ip2.String())
	}
	ip3, err := record.Alloc(id2)
	if err != nil {
		t.Fatalf("Alloc Failed")
	}
	if equalIPNet(*ip1, *ip3) {
		t.Fatalf("Ip Equal")
	}
	data, _ := json.Marshal(record)
	log.Log.Debugf("[json]%s", data)
}
func TestRecord_Release(t *testing.T) {
}
