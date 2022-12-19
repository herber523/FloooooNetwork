package node

import (
	"fmt"
	mrand "math/rand"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
)

func NewNode(seed int64, port string) (host.Host, error) {
	r := mrand.New(mrand.NewSource(int64(seed)))
	fmt.Println("Seed:", seed)
	fmt.Println("R:", r)
	privateKey, _, err := crypto.GenerateKeyPairWithReader(crypto.RSA, 2048, r)
	ip4quic := fmt.Sprintf("/ip4/0.0.0.0/udp/%s/quic", port)
	fmt.Println(ip4quic)
	node, err := libp2p.New(
		libp2p.ListenAddrStrings(ip4quic),
		libp2p.DefaultSecurity,
		libp2p.Identity(privateKey),
		libp2p.NATPortMap(),
		libp2p.DefaultMuxers,
		libp2p.FallbackDefaults,
	)

	return node, err
}
