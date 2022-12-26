package devices

import (
	"blitz/pkg/hardware"
	"blitz/pkg/ipnet"
	"testing"
)

func TestGetBridge(t *testing.T) {
	cidr, err := ipnet.ParseCIDR("192.168.35.1/24")
	if err != nil {
		t.Fatal(err)
	}
	br, err := GetBridge(cidr)
	if err != nil {
		t.Fatal(err)
	}
	if br == nil {
		t.Fatal("invalid devices")
	}
}
func TestGetHostIP(t *testing.T) {
	//TODO: ADD IPv6 Support
	subnet, err := GetHostIP(IPv4)
	if err != nil {
		t.Fatal()
	}
	links, err := LinkByIP(subnet)
	if err != nil {
		t.Logf("err:%s", err)
	}
	t.Logf("Links:%s", links.Attrs().Name)
}
func TestGenHardwareAddr(t *testing.T) {
	addr := hardware.GenHardwareAddr()
	t.Log(addr.ToNetHardwareAddr().String())
}
func TestSetupVXLAN(t *testing.T) {
	_, err := ipnet.ParseCIDR("10.20.1.1/24")
	if err != nil {
		t.Fatal(err)
	}
	//err = SetupVXLAN(*cidr)
	//if err != nil {
	//	t.Fatal(err)
	//}

}
