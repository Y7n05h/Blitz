package ipnet

import (
	"net"
	"testing"
)

func TestIPNet_Contains(t *testing.T) {
	cidr1, _ := ParseCIDR("192.168.1.0/24")
	cidr2, _ := ParseCIDR("192.168.1.127/24")
	{
		ip := net.ParseIP("192.168.1.0")
		if cidr1.Contains(ip) != cidr2.Contains(ip) {
			t.Errorf("Error")
		}
	}
	{
		ip := net.ParseIP("192.168.1.1")
		if cidr1.Contains(ip) != cidr2.Contains(ip) {
			t.Errorf("Error")
		}
	}
	{
		ip := net.ParseIP("192.168.1.127")
		if cidr1.Contains(ip) != cidr2.Contains(ip) {
			t.Errorf("Error")
		}
	}
	{
		ip := net.ParseIP("192.168.1.255")
		if cidr1.Contains(ip) != cidr2.Contains(ip) {
			t.Errorf("Error")
		}
	}
}
