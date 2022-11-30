package bridge

import (
	"crypto/rand"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/ipnet"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/vishvananda/netlink"
)

func CheckLinkContainIPNNet(gateway *ipnet.IPNet, br netlink.Link) bool {
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
	for _, v := range address {
		if gateway.Equal(ipnet.FromNetIPNet(v.IPNet)) {
			return true
		}
	}
	return false
}
func GetBridge(gateway *ipnet.IPNet) (netlink.Link, error) {
	var linkErr netlink.LinkNotFoundError
	if br, err := netlink.LinkByName(constexpr.BridgeName); err == nil {
		if br != nil && CheckLinkContainIPNNet(gateway, br) {
			return br, nil
		}
		log.Log.Fatal("Not Expect Link: gateway not same: expect ip:%s", gateway.String())
	} else if !errors.As(err, &linkErr) {
		log.Log.Warnf("Not Expect Link Error:%v", err)
		return nil, err
	}
	br, err := netlink.LinkByName(constexpr.BridgeName)
	if err != nil {
		if !errors.As(err, &linkErr) {
			log.Log.Warnf("Not Expect Link Error:%v", err)
			return nil, err
		}
	} else {
		if br != nil {
			log.Log.Error("Not Expect Link: gateway not same")
			goto newBr
		}
		if CheckLinkContainIPNNet(gateway, br) {
			return br, nil
		}
		goto addAddr
	}
newBr:
	br = &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name:   constexpr.BridgeName,
			MTU:    constexpr.Mtu,
			TxQLen: -1,
		},
	}
addAddr:
	if err := netlink.LinkAdd(br); err != nil && err != syscall.EEXIST {
		return nil, err
	}

	dev, err := netlink.LinkByName(constexpr.BridgeName)
	if err != nil {
		return nil, err
	}

	if err := netlink.AddrAdd(dev, &netlink.Addr{IPNet: gateway.ToNetIPNet()}); err != nil {
		return nil, err
	}

	if err := netlink.LinkSetUp(dev); err != nil {
		return nil, err
	}
	return dev, nil
}
func SetupVeth(netns ns.NetNS, br netlink.Link, ifName string, podIP *ipnet.IPNet, gateway net.IP) error {
	hostIdx := -1
	err := netns.Do(func(hostNS ns.NetNS) error {
		// setup lo, kubernetes will call loopback internal
		loLink, err := netlink.LinkByName("lo")
		if err != nil {
			log.Log.Debugf("Get lo Failed")
			return err
		}

		if err := netlink.LinkSetUp(loLink); err != nil {
			return err
		}
		log.Log.Debugf("Set lo up")
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
		if err := netlink.AddrAdd(conLink, &netlink.Addr{IPNet: podIP.ToNetIPNet()}); err != nil {
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
		log.Log.Fatal("Error:%v %#v", err, err)
		return err
	}
	log.Log.Debugf("Create Veth Success")
	// need to lookup hostVeth again as its index has changed during ns move
	hostVeth, err := netlink.LinkByIndex(hostIdx)
	if err != nil {
		return fmt.Errorf("failed to lookup %d: %v", hostIdx, err)
	}
	log.Log.Debugf("Get HostVeth Success1")
	if hostVeth == nil {
		return fmt.Errorf("nil hostveth")
	}
	log.Log.Debugf("Get HostVeth Success1")

	// connect host veth end to the bridge
	if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("failed to connect %q to bridge %v: %v", hostVeth.Attrs().Name, br.Attrs().Name, err)
	}
	log.Log.Debugf("Connect Link and br Success")
	return nil
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
func LinkByIP(ip *ipnet.IPNet) (netlink.Link, error) {
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
func SetupVXLAN(bridge netlink.Link) error {
	_, err := netlink.LinkByName(constexpr.VXLANName)
	var linkErr netlink.LinkNotFoundError
	if err == nil {
		log.Log.Debugf("Found VXLAN exist")
		return nil
	} else if !errors.As(err, &linkErr) {
		log.Log.Error("No Expect Error: ", err)
		return err
	}
	gatewayLink, err := GetDefaultGateway()
	if err != nil {
		return err
	}
	log.Log.Debugf("Get Default Gateway Success")
	vxlan, err := createVXLAN((*gatewayLink).Attrs().Index)
	err = netlink.LinkSetMaster(vxlan, bridge)
	return err
}
func createVXLAN(ifIdx int) (*netlink.Vxlan, error) {
	group := net.ParseIP(constexpr.VXLANGroup)
	if group == nil {
		log.Log.Fatal("Parse Failed")
	} else {
		log.Log.Debugf("group:%s", group.String())
	}

	var linkErr netlink.LinkNotFoundError
	exist, err := netlink.LinkByName(constexpr.VXLANName)
	if errors.As(err, &linkErr) && exist != nil {
		log.Log.Infof("VXLAN exist")
		return exist.(*netlink.Vxlan), nil
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = constexpr.VXLANName
	attrs.HardwareAddr = GenHardwareAddr()
	vtep := netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      constexpr.VxlanId,
		VtepDevIndex: ifIdx,
		Group:        group,
		Learning:     true,
		Port:         constexpr.VXLANPort,
	}
	log.Log.Debugf("VXLAN: mac addr:%s", vtep.Attrs().HardwareAddr.String())
	err = netlink.LinkAdd(&vtep)
	if err != nil {
		log.Log.Warnf("Error %v: Create VXLAN failed: %#v", err, vtep)
		return nil, err
	}
	vxlan, err := netlink.LinkByName(attrs.Name)
	if err != nil {
		log.Log.Warn("Found VXLAN Failed", err)
		return nil, err
	}
	err = netlink.LinkSetUp(vxlan)
	if err != nil {
		log.Log.Errorf("set VXLAN up Failed:%v", err)
		return nil, err
	}
	return vxlan.(*netlink.Vxlan), nil
}
func GetHostIP() (*ipnet.IPNet, error) {
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
		return ipnet.FromNetIPNet(addr.IPNet), nil
	}
	return nil, fmt.Errorf("get HostIP failed")
}

func GenHardwareAddr() net.HardwareAddr {
	mac := make([]byte, 6)
	_, err := rand.Read(mac)
	if err != nil {
		log.Log.Fatal("Gen Mac Addr Failed")
	}
	mac[0] &= 0xfe
	mac[0] |= 0x2
	return mac
}
