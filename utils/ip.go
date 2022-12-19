package utils

import (
	"crypto/rand"
	"net"
)

func RandomIPFromRange() (net.IP, error) {
	ip, ipnet, err := net.ParseCIDR("100.64.0.0/16")
	if err != nil {
		return nil, err
	}
	for {
		// The number of leading 1s in the mask
		ones, _ := ipnet.Mask.Size()
		quotient := ones / 8
		remainder := ones % 8

		// create random 4-byte byte slice
		r := make([]byte, 4)
		rand.Read(r)

		for i := 0; i <= quotient; i++ {
			if i == quotient {
				shifted := byte(r[i]) >> remainder
				r[i] = ^ipnet.IP[i] & shifted
			} else {
				r[i] = ipnet.IP[i]
			}
		}
		ip = net.IPv4(r[0], r[1], r[2], r[3])

		if !ip.Equal(ipnet.IP) {
			return ip, nil
		}
	}
}
