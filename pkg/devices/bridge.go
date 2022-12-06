package devices

import (
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"
	"tiny_cni/internal/constexpr"
	"tiny_cni/internal/log"
	"tiny_cni/pkg/hardware"
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
func SetupVeth(netns ns.NetNS, br netlink.Link, ifName string, podIP *ipnet.IPNet, gateway net.IP, clusterCIDR *ipnet.IPNet) error {
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
		if err != nil {
			return err
		}
		err = ip.AddRoute(clusterCIDR.ToNetIPNet(), gateway, conLink)
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

	// connect host veth end to the devices
	if err := netlink.LinkSetMaster(hostVeth, br); err != nil {
		return fmt.Errorf("failed to connect %q to devices %v: %v", hostVeth.Attrs().Name, br.Attrs().Name, err)
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
func SetupVXLAN(subnet *ipnet.IPNet) (*netlink.Vxlan, error) {
	_, err := netlink.LinkByName(constexpr.VXLANName)
	var linkErr netlink.LinkNotFoundError
	if err == nil {
		log.Log.Debugf("Found VXLAN exist")
		return nil, nil
	} else if !errors.As(err, &linkErr) {
		log.Log.Error("No Expect Error: ", err)
		return nil, err
	}
	gatewayLink, err := GetDefaultGateway()
	if err != nil {
		return nil, err
	}
	log.Log.Debugf("Get Default Gateway Success")
	vxlan, err := createVXLAN((*gatewayLink).Attrs().Index)
	if err != nil {
		log.Log.Error("createVXLAN Failed:", err)
	}
	err = netlink.AddrAdd(vxlan, &netlink.Addr{IPNet: subnet.ToNetIPNet()})
	return vxlan, err
}
func createVXLAN(ifIdx int) (*netlink.Vxlan, error) {
	var linkErr netlink.LinkNotFoundError
	exist, err := netlink.LinkByName(constexpr.VXLANName)
	if errors.As(err, &linkErr) && exist != nil {
		log.Log.Infof("VXLAN exist")
		return exist.(*netlink.Vxlan), nil
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = constexpr.VXLANName
	addr := hardware.GenHardwareAddr()
	attrs.HardwareAddr = addr.ToNetHardwareAddr()
	vtep := netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      constexpr.VxlanId,
		VtepDevIndex: ifIdx,
		Learning:     false,
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
func GetRouteByDist(idx int, subnet ipnet.IPNet) *netlink.Route {
	route := netlink.Route{
		LinkIndex: idx,
		Dst:       subnet.ToNetIPNet(),
	}
	res, err := netlink.RouteListFiltered(netlink.FAMILY_V4, &route, netlink.RT_FILTER_DST|netlink.RT_FILTER_OIF)
	if err != nil {
		log.Log.Debugf("Can not get route idx:%d subnet:%s", idx, subnet.String())
		return nil
	}
	if len(res) == 0 {
		log.Log.Debugf("Can not get route idx:%d subnet:%s", idx, subnet.String())
		return nil
	}
	if len(res) > 1 {
		log.Log.Errorf("Error Multi Route:%#v", res)
	}
	return &res[0]
}
func GetNeighByIP(idx int, ip net.IP) *netlink.Neigh {
	neighs, err := netlink.NeighList(idx, netlink.FAMILY_V4)
	if err != nil {
		log.Log.Error("Get Neigh Failed:", err)
		return nil
	}
	for _, neigh := range neighs {
		if neigh.IP.Equal(ip) {
			return &neigh
		}
	}
	log.Log.Error("No Target Neigh")
	return nil
}

func AddFDB(ifIdx int, ip net.IP, address hardware.Address) error {
	log.Log.Infof("calling AddFDB: %v, %v", ip, address.ToNetHardwareAddr().String())
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    ifIdx,
		State:        netlink.NUD_PERMANENT,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           ip,
		HardwareAddr: address.ToNetHardwareAddr(),
	})
}
func DelFDB(ifIdx int, ip net.IP, address hardware.Address) error {
	log.Log.Infof("calling DelFDB: %v, %v", ip, address.ToNetHardwareAddr().String())
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    ifIdx,
		Family:       syscall.AF_BRIDGE,
		Flags:        netlink.NTF_SELF,
		IP:           ip,
		HardwareAddr: address.ToNetHardwareAddr(),
	})
}

func AddARP(ifIdx int, ip net.IP, address hardware.Address) error {
	log.Log.Infof("calling AddARP: %v, %v", ip, address.ToNetHardwareAddr().String())
	return netlink.NeighSet(&netlink.Neigh{
		LinkIndex:    ifIdx,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           ip,
		HardwareAddr: address.ToNetHardwareAddr(),
	})
}

func DelARP(ifIdx int, ip net.IP, address hardware.Address) error {
	log.Log.Infof("calling DelARP: %v, %v", ip, address.ToNetHardwareAddr().String())
	return netlink.NeighDel(&netlink.Neigh{
		LinkIndex:    ifIdx,
		State:        netlink.NUD_PERMANENT,
		Type:         syscall.RTN_UNICAST,
		IP:           ip,
		HardwareAddr: address.ToNetHardwareAddr(),
	})
}
