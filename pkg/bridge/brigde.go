package bridge

import (
	"fmt"
	"net"
	"syscall"

	"github.com/vishvananda/netlink"
)

const (
	bridgeName = "tcni0"
	mtu        = 1500
)

func GetBridge(gateway *net.IPNet) (netlink.Link, error) {
	if br, err := netlink.LinkByName(bridgeName); err != nil {
		return nil, err
	} else {
		if br != nil {
			ipFamily := 0
			if gateway.IP.To4() != nil {
				ipFamily = syscall.AF_INET
			} else {
				ipFamily = syscall.AF_INET6
			}

			addrs, err := netlink.AddrList(br, ipFamily)
			if err != nil {
				return nil, fmt.Errorf("can not get ip list from %s", bridgeName)
			}
			for _, v := range addrs {

			}
			//return br, nil
		}
	}

	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   bridgeName,
			MTU:    mtu,
			TxQLen: -1,
		},
	}

	if err := netlink.LinkAdd(br); err != nil && err != syscall.EEXIST {
		return nil, err
	}

	dev, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return nil, err
	}

	if err := netlink.AddrAdd(dev, &netlink.Addr{IPNet: gateway}); err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(dev); err != nil {
		return nil, err
	}
	return dev, nil
}
