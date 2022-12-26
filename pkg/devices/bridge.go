package devices

import (
	"blitz/pkg/constant"
	"blitz/pkg/hardware"
	"blitz/pkg/ipnet"
	"blitz/pkg/log"
	"errors"
	"fmt"
	"net"
	"os"
	"syscall"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"

	"github.com/vishvananda/netlink"
)

const (
	IPv4 = netlink.FAMILY_V4
	IPv6 = netlink.FAMILY_V6
)

func CheckLinkContainIPNNet(gateway []*ipnet.IPNet, br netlink.Link) bool {
	address, err := netlink.AddrList(br, netlink.FAMILY_ALL)
	if err != nil {
		log.Log.Debugf("Get Link Address Failed:%v", err)
		return false
	}
	cidrs := make(map[string]bool)
	for _, cidr := range address {
		cidrs[cidr.String()] = true
	}
	for _, v := range gateway {
		_, ok := cidrs[v.String()]
		if !ok {
			return false
		}
	}
	return true
}
func GetBridge(gateway []*ipnet.IPNet) (netlink.Link, error) {
	var linkErr netlink.LinkNotFoundError
	if br, err := netlink.LinkByName(constant.BridgeName); err == nil {
		if br != nil && CheckLinkContainIPNNet(gateway, br) {
			return br, nil
		}
		log.Log.Fatalf("Not Expect Link: gateway not same: expect ip:%v", gateway)
	} else if !errors.As(err, &linkErr) {
		log.Log.Warnf("Not Expect Link Error:%v", err)
		return nil, err
	}
	br, err := netlink.LinkByName(constant.BridgeName)
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
			Name:   constant.BridgeName,
			MTU:    constant.Mtu,
			TxQLen: -1,
		},
	}
	log.Log.Debug("No Bridge Exist.Try to create one.")
	if err := netlink.LinkAdd(br); err != nil && err != syscall.EEXIST {
		log.Log.Error("Try to create bridge Failed.")
		return nil, err
	}

addAddr:
	dev, err := netlink.LinkByName(constant.BridgeName)
	if err != nil {
		log.Log.Error("Get Bridge Failed")
		return nil, err
	}
	for _, subnet := range gateway {
		if err := netlink.AddrAdd(dev, &netlink.Addr{IPNet: subnet.ToNetIPNet()}); err != nil {
			if err == syscall.EEXIST {
				continue
			}
			log.Log.Errorf("Add Addr Failed.Err:%v,%#v", err, err)
			return nil, err
		}
	}

	if err := netlink.LinkSetUp(dev); err != nil {
		log.Log.Error("Set Link Up failed")
		return nil, err
	}
	return dev, nil
}

type NetworkInfo struct {
	PodIP       ipnet.IPNet
	Gateway     ipnet.IPNet
	ClusterCIDR ipnet.IPNet
}

func SetupVeth(netns ns.NetNS, br netlink.Link, ifName string, info []NetworkInfo) error {
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
		host, container, err := ip.SetupVeth(ifName, constant.Mtu, "", hostNS)
		if err != nil {
			log.Log.Errorf("Setup Veth Error:%v %#v", err, err)
			return err
		}
		hostIdx = host.Index

		// set ip for container veth
		conLink, err := netlink.LinkByIndex(container.Index)
		if err != nil {
			log.Log.Errorf("Link By Index Error:%v %#v", err, err)
			return err
		}
		for _, i := range info {
			log.Log.Debugf("Setup Container Veth: PodIP: %s Gateway:%s ClusterCIDR:%s", i.PodIP.String(), i.Gateway.String(), i.ClusterCIDR.String())
			if err := netlink.AddrAdd(conLink, &netlink.Addr{IPNet: i.PodIP.ToNetIPNet()}); err != nil {
				log.Log.Errorf("Add Addr For Veth Error:%v %#v", err, err)
				return err
			}
			// add default route
			if err := ip.AddDefaultRoute(i.Gateway.IP, conLink); err != nil {
				log.Log.Errorf("Add Default Route Error:%v %#v", err, err)
				return err
			}
			if err := ip.AddRoute(i.ClusterCIDR.ToNetIPNet(), i.Gateway.IP, conLink); err != nil {
				log.Log.Errorf("Add Route for veth Error:%v %#v", err, err)
				return err
			}
		}
		// setup container veth
		if err = netlink.LinkSetUp(conLink); err != nil {
			log.Log.Errorf("Link Set Up Error:%v %#v", err, err)
			return nil
		}
		return nil
	})
	if err != nil {
		log.Log.Fatalf("Error:%v %#v", err, err)
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
		if CheckLinkContainIPNNet([]*ipnet.IPNet{ip}, link) {
			return link, err
		}
	}
	return nil, fmt.Errorf("can not found ip")
}
func GetDefaultGateway(family int) (*netlink.Link, error) {
	routes, err := netlink.RouteList(nil, family)
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
	link, err := netlink.LinkByName(constant.VXLANName)
	var linkErr netlink.LinkNotFoundError
	if err == nil {
		log.Log.Debugf("Found VXLAN exist")
		return link.(*netlink.Vxlan), nil
	} else if !errors.As(err, &linkErr) {
		log.Log.Error("No Expect Error: ", err)
		return nil, err
	}
	//TODO: ADD IPv6 Support
	gatewayLink, err := GetDefaultGateway(IPv4)
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
	exist, err := netlink.LinkByName(constant.VXLANName)
	if errors.As(err, &linkErr) && exist != nil {
		log.Log.Infof("VXLAN exist")
		return exist.(*netlink.Vxlan), nil
	}
	attrs := netlink.NewLinkAttrs()
	attrs.Name = constant.VXLANName
	addr := hardware.GenHardwareAddr()
	attrs.HardwareAddr = addr.ToNetHardwareAddr()
	vtep := netlink.Vxlan{
		LinkAttrs:    attrs,
		VxlanId:      constant.VxlanId,
		VtepDevIndex: ifIdx,
		Learning:     false,
		Port:         constant.VXLANPort,
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
func GetHostIP(family int) (*ipnet.IPNet, error) {
	link, err := GetDefaultGateway(family)
	if err != nil {
		return nil, err
	}
	address, err := netlink.AddrList(*link, family)
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
