package ipam

import (
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"encoding/json"
	"fmt"
	"testing"
)

func checkIPNetUnderSubnet(t *testing.T, ipNet ipnet.IPNet, subnet ipnet.IPNet) {
	if !subnet.ToNetIPNet().Contains(ipNet.IP) {
		t.Fatalf("Alloc IP Error")
	}
	if ipNet.Mask.String() != subnet.Mask.String() {
		t.Fatalf("Alloc IP Mask Error")
	}
}
func TestRecord_Alloc1(t *testing.T) {
	subnet, _ := ipnet.ParseCIDR("192.168.1.0/29")
	record := New(subnet)
	ips := make([]*ipnet.IPNet, 0)
	for i := 0; i < 5; i++ {
		ip, err := record.Alloc(fmt.Sprintf("%d", i))
		if err != nil {
			t.Errorf("Alloc Error")
		}
		ips = append(ips, ip)
	}
	if _, err := record.Alloc("8"); err == nil {
		t.Errorf("Alloc Error")
	}
	if err := record.Release("3"); err != nil {
		t.Errorf("Release Error")
	}
	if ip, err := record.Alloc("8"); err != nil {
		t.Errorf("Alloc Error")
	} else {
		if !ip.Equal(ips[3]) {
			t.Errorf("Alloc after Release Error")
		}
	}
	t.Logf("ips:%v", ips)
}
func TestRecord_Alloc2(t *testing.T) {
	subnet, _ := ipnet.ParseCIDR("192.168.1.0/24")
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
func TestRecord_UnmarshalJSON(t *testing.T) {
	subnet := ipnet.IPNet{}
	cidrString := "192.168.1.1/24"
	err := subnet.UnmarshalJSON([]byte("\"" + cidrString + "\""))
	if err != nil {
		log.Log.Errorf("UnmarshalJSON Failed:%v", err)
	}
	if cidrString != subnet.String() {
		t.Fatalf("UnmarshalJSON Err: expect:%s    really:%s", cidrString, subnet.String())
	}
}
