package bridge

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/vishvananda/netlink"
)

func CheckLinkContainIPNNet(gateway *net.IPNet, br netlink.Link) bool {
	ipFamily := 0
	if gateway.IP.To4() != nil {
		ipFamily = netlink.FAMILY_V4
	} else {
		ipFamily = netlink.FAMILY_V6
	}
	address, err := netlink.AddrList(br, ipFamily)
	if err != nil {
		return false
	}
	//TODO: check Mask
	for _, v := range address {
		if v.IP.Equal(gateway.IP) {
			return true
		}
	}
	return false
}
func GetBridge(gateway *net.IPNet) (netlink.Link, error) {
	var linkErr netlink.LinkNotFoundError
	if br, err := netlink.LinkByName(constexpr.BridgeName); err == nil {
		if br != nil && CheckLinkContainIPNNet(gateway, br) {
			return br, nil
		}
		log.Log.Fatal("Not Expect Link: gateway not same")
	} else if !errors.As(err, &linkErr) {
		log.Log.Warnf("Not Expect Link Error:%v", err)
		return nil, err
	}
	log.Log.Debug("Bridge: No Bridge, Create New Bridge")
	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   constexpr.BridgeName,
			MTU:    constexpr.Mtu,
			TxQLen: -1,
		},
	}

	if err := netlink.LinkAdd(br); err != nil && err != syscall.EEXIST {
		return nil, err
	}

	dev, err := netlink.LinkByName(constexpr.BridgeName)
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
func SetupVeth(netns ns.NetNS, br netlink.Link, ifName string, podIP *net.IPNet, gateway net.IP) error {
	hostIdx := -1
	err := netns.Do(func(hostNS ns.NetNS) error {
		// setup lo, kubernetes will call loopback internal
		loLink, err := netlink.LinkByName("lo")
		if err != nil {
			return err
		}

		if err := netlink.LinkSetUp(loLink); err != nil {
			return err
		}

		// create the veth pair in the container and move host end into host netns
		host, container, err := ip.SetupVeth(ifName, constexpr.Mtu, "", hostNS)
		if err != nil {
			return err
		}
		hostIdx = host.Index

		// set ip for container veth
		conLink, err := netlink.LinkByIndex(container.Index)
		if err != nil {
			return err
		}
		if err := netlink.AddrAdd(conLink, &netlink.Addr{IPNet: podIP}); err != nil {
			return err
		}

		// setup container veth
		if err := netlink.LinkSetUp(conLink); err != nil {
			return err
		}

		// add default route
		if err := ip.AddDefaultRoute(gateway, conLink); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByIndex(hostIdx)
	if err != nil {
		return fmt.Errorf("failed to lookup %d: %v", hostIdx, err)
	}

	if hostVeth == nil {
		return fmt.Errorf("nil hostveth")
	}

	// connect host veth end to the bridge
	if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", hostVeth.Attrs().Name, br.Attrs().Name, err)
	}

	hostIP, err := GetHostIP()
	if err != nil {
		return err
	}
	_, err = CreateVXLAN(hostIP.IP)
	return err
}
func DelVeth(netns ns.NetNS, ifName string) error {
	return netns.Do(func(_ ns.NetNS) error {
		veth, err := netlink.LinkByName(ifName)
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		if err != nil {
			log.Log.Errorf("Get Link Error:%#v", err)
			return err
		}
		if veth == nil {
			log.Log.Errorf("invlid veth ifName:%s", ifName)
			return nil
		}
		err = netlink.LinkDel(veth)
		if err != nil {
			log.Log.Errorf("Del veth(%s):%#v failed:%#v ", ifName, veth.Attrs(), err)
		}
		return nil
	})
}
func LinkByIP(ip *net.IPNet) (netlink.Link, error) {
	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link == nil {
			continue
		}
		if CheckLinkContainIPNNet(ip, link) {
			return link, err
		}
	}
	return nil, fmt.Errorf("can not found ip")
}
func GetDefaultGateway() (*netlink.Link, error) {
	//	TODO: Support IPv6
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	for _, route := range routes {
		if route.Dst == nil {
			link, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				log.Log.Warn("LinkByIndex:", err)
				continue
			}
			return &link, nil
		}
	}
	return nil, fmt.Errorf("get Default Gateway failed")
}
func CreateVXLAN(hostIP net.IP) (*netlink.Vxlan, error) {
	group, _, _ := net.ParseCIDR(constexpr.VXLANGroup)
	attrs := netlink.NewLinkAttrs()
	attrs.Name = constexpr.VXLANName

	exist, err := netlink.LinkByName(constexpr.VXLANName)
	if err == os.ErrNotExist && exist != nil {
		return exist.(*netlink.Vxlan), nil
	}
	vtep := netlink.Vxlan{
		LinkAttrs: attrs,
		VxlanId:   constexpr.VxlanId,
		SrcAddr:   hostIP,
		Group:     group,
		Learning:  true,
		Port:      constexpr.VXLANPort,
	}
	err = netlink.LinkAdd(&vtep)
	if err != nil {
		return nil, err
	}
	vxlan, err := netlink.LinkByName(attrs.Name)
	if err != nil {
		return nil, err
	}
	return vxlan.(*netlink.Vxlan), nil
}
func GetHostIP() (*net.IPNet, error) {
	link, err := GetDefaultGateway()
	if err != nil {
		return nil, err
	}
	//	TODO: Support IPv6
	address, err := netlink.AddrList(*link, netlink.FAMILY_V4)
	if err != nil {
		return nil, err
	}
	for _, addr := range address {
		return addr.IPNet, nil
	}
	return nil, fmt.Errorf("get HostIP failed")
}
