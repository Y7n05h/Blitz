package ipam

import (
	"encoding/json"
	"testing"
	"tiny_cni/internal/ipnet"
	"tiny_cni/internal/log"
)

func checkIPNetUnderSubnet(t *testing.T, ipNet ipnet.IPNet, subnet ipnet.IPNet) {
	if !subnet.ToNetIPNet().Contains(ipNet.IP) {
		t.Fatalf("Alloc IP Error")
	}
	if ipNet.Mask.String() != subnet.Mask.String() {
		t.Fatalf("Alloc IP Mask Error")
	}
}
func TestRecord_Alloc(t *testing.T) {
	subnet, _ := ipnet.ParseCIDR("192.168.1.1/24")
	record := New(subnet, subnet)
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
	if !ip1.Equal(ip2) {
		t.Fatalf("Ip not Equal: ip1 %s ip2 %s", ip1.String(), ip2.String())
	}
	ip3, err := record.Alloc(id2)
	if err != nil {
		t.Fatalf("Alloc Failed")
	}
	if ip1.Equal(ip3) {
		t.Fatalf("Ip Equal")
	}
	data, _ := json.Marshal(record)
	log.Log.Debugf("[json]%s", data)
}
func TestRecord_Release(t *testing.T) {
}
