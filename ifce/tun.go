package ifce

import (
	"log"
	"net"

	"github.com/songgao/water"
	"github.com/vishvananda/netlink"
)

func SetTun(ip string, mask int) (*water.Interface, error) {
	config := water.Config{
		DeviceType: water.TUN,
	}

	ifceTap, err := water.New(config)
	if err != nil {
		log.Fatal(err)
	}
	addr := &netlink.Addr{
		IPNet: &net.IPNet{
			IP:   net.ParseIP(ip),
			Mask: net.CIDRMask(mask, 32),
		},
	}
	ifceLink, _ := netlink.LinkByName(ifceTap.Name())
	if err := netlink.AddrAdd(ifceLink, addr); err != nil {
		log.Fatal(err)
	}
	netlink.LinkSetUp(ifceLink)
	return ifceTap, nil
}
