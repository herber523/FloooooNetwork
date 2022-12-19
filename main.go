package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-datastore"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/songgao/packets/ethernet"
	"github.com/songgao/water"

	"p2p/ifce"
	pn "p2p/node"
	"p2p/utils"
)

var (
	MyIp        string = ""
	PeerIpTable        = make(map[string]peer.ID)
	StreamTable        = make(map[string]network.Stream)
	Ifce        *water.Interface
)

func initRoutableIp(node host.Host, myIp *string) {
	ip, _ := utils.RandomIPFromRange()
	PeerIpTable[ip.String()] = node.ID()
	*myIp = ip.String()
}

func joinToAdmin(ctx context.Context, node host.Host, dhtOut *dht.IpfsDHT, adminHost *string) error {
	pid, _ := peer.Decode(*adminHost)

	node.Network().Connectedness(pid)
	addrs, err := dhtOut.FindPeer(ctx, pid)
	if err != nil {
		fmt.Println(err)
	}
	_, err = node.Network().DialPeer(ctx, addrs.ID)
	if err != nil {
		fmt.Println(err)
	}
	// retry create stream 5 times
	var stream network.Stream
	for {

		stream, err = node.NewStream(ctx, pid, "/join/1.0.0")
		fmt.Println(pid)
		if err != nil {
			fmt.Println(err)
		}
		if stream != nil {
			fmt.Println("stream created")
			fmt.Println(stream)
			break
		}
		time.Sleep(5 * time.Second)
	}

	_, err = stream.Write([]byte("join\n"))
	if err != nil {
		fmt.Println(err)
	}
	return nil

}

func streamTable(ctx context.Context, node host.Host, dhtOut *dht.IpfsDHT, streamTable *map[string]network.Stream, peerIpTable *map[string]peer.ID) {
	for {
		fmt.Println("Upgrading stream table")
		for ip, pid := range *peerIpTable {
			fmt.Println("IP:", ip, "PeerID:", pid)
			if _, ok := (*streamTable)[ip]; ok {
				fmt.Println("Stream already exists")
				continue
			}
			if pid == node.ID() {
				fmt.Println("This is me")
				continue
			}
			addrs, err := dhtOut.FindPeer(ctx, pid)
			if err != nil {
				fmt.Println("find peer", err)
			}
			_, err = node.Network().DialPeer(ctx, addrs.ID)
			if err != nil {
				fmt.Println("dial peer", err)
			}
			stream, err := node.NewStream(ctx, pid, "/vpn/1.0.0")
			if err != nil {
				fmt.Println(err)
			} else {
				(*streamTable)[ip] = stream
			}
		}
		fmt.Println("StreamTable:", *streamTable)

		time.Sleep(1 * time.Second)
	}
}

func vpnStream(stream network.Stream) {
	var frame ethernet.Frame
	fmt.Println("vpnStream")
	for {
		frame.Resize(1420)
		n, err := stream.Read(frame)
		if err != nil {
			fmt.Println(err)
		}
		frame = frame[:n]
		//fmt.Println("frame:", frame)
		Ifce.Write(frame)
		if err != nil {
			fmt.Println(err)
			// delete stream from table
			break
		}
	}
}

func routeTableStream(stream network.Stream) {
	r := bufio.NewReader(stream)
	for {
		str, _ := r.ReadString('\n')
		if str == "0x00\n" {
			break
		}
		fmt.Println(str)
		data := strings.Split(str, "|")
		fmt.Println(data)
		ip := data[0]
		id := strings.Split(data[1], "\n")[0]

		pid, _ := peer.Decode(id)
		PeerIpTable[ip] = pid
		if pid == stream.Conn().LocalPeer() {
			MyIp = ip
		}
	}
	fmt.Println("PeerIpTable:", PeerIpTable)
}

// create a handler for stream
func joinStream(stream network.Stream) {
	pid := stream.Conn().RemotePeer()
	// get stream's source peer ID
	r := bufio.NewReader(stream)
	for {
		str, _ := r.ReadString('\n')
		fmt.Println(str)
		if str == "join\n" {
			fmt.Printf("Peer %s joined the chat", pid)
			ip, _ := utils.RandomIPFromRange()
			PeerIpTable[ip.String()] = pid
		}
		break
	}
	fmt.Println("PeerIpTable:", PeerIpTable)
}

func discover(ctx context.Context, node host.Host, dhtOut *dht.IpfsDHT) {
	for {
		time.Sleep(5 * time.Second)
		fmt.Println("Host ID is", node.ID())
		// loop peerIpTable
		for ip, pid := range PeerIpTable {
			if pid == node.ID() {
				continue
			}
			fmt.Println("IP:", ip, "PeerID:", pid)

			node.Network().Connectedness(pid)
			addrs, err := dhtOut.FindPeer(ctx, pid)
			if err != nil {
				fmt.Println(err)
			}
			_, err = node.Network().DialPeer(ctx, addrs.ID)
			if err != nil {
				fmt.Println(err)
			}
			stream, err := node.NewStream(ctx, pid, "/route/1.0.0")
			if err != nil {
				fmt.Println(err)
			}
			for ip_, pid_ := range PeerIpTable {
				pidIp := fmt.Sprintf("%s|%s\n", ip_, pid_.Pretty())
				stream.Write([]byte(pidIp))
			}
			stream.Write([]byte("0x00\n"))

			stream.Close()
		}
	}
}

func initIfce(myIp *string) (*water.Interface, error) {
	for {
		if *myIp == "" {
			time.Sleep(1 * time.Second)
			continue
		}
		fmt.Println("MyIp:", MyIp)
		ifceTap, err := ifce.SetTun(*myIp, 16)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("ifceTap:", ifceTap.Name())
		return ifceTap, err
	}
}

func main() {
	otherHost := flag.String("other-host", "", "The hostname of the other host")
	seed := flag.Int64("seed", 0, "The seed to use for the random number generator")
	port := flag.String("port", "", "The port to use for the connection")
	admin := flag.Bool("admin", false, "Whether to run the admin server")

	flag.Parse()

	fmt.Println("other-host:", *otherHost)
	node, err := pn.NewNode(*seed, *port)
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	dhtOut := dht.NewDHTClient(ctx, node, datastore.NewMapDatastore())
	go streamTable(ctx, node, dhtOut, &StreamTable, &PeerIpTable)

	BootstrapPeers := dht.GetDefaultBootstrapPeerAddrInfos()
	var wg sync.WaitGroup
	lock := sync.Mutex{}
	count := 0
	wg.Add(len(BootstrapPeers))
	for _, peerInfo := range BootstrapPeers {
		go func(peerInfo peer.AddrInfo) {
			defer wg.Done()
			err := node.Connect(ctx, peerInfo)
			if err == nil {
				lock.Lock()
				count++
				lock.Unlock()

			}
		}(peerInfo)
	}
	wg.Wait()

	fmt.Println("Connected to", count, "bootstrap nodes")
	fmt.Println("Host ID is", node.ID())
	fmt.Println(node.Addrs())

	if *admin {
		initRoutableIp(node, &MyIp)

		node.SetStreamHandler("/join/1.0.0", joinStream)

		go discover(ctx, node, dhtOut)

	} else {
		node.SetStreamHandler("/route/1.0.0", routeTableStream)
		joinToAdmin(ctx, node, dhtOut, otherHost)

	}
	node.SetStreamHandler("/vpn/1.0.0", vpnStream) // todo

	Ifce, err = initIfce(&MyIp)
	if err != nil {
		panic(err)
	}

	go func() {
		var frame ethernet.Frame

		for {
			frame.Resize(1420)
			n, err := Ifce.Read([]byte(frame))
			if err != nil {
				log.Println(err)
			}
			dst := net.IPv4(frame[16], frame[17], frame[18], frame[19]).String()
			dstStr := net.IP(dst).String()
			frame = frame[:n]
			//fmt.Println("frame:", frame)
			// get dst ip

			// find stream by dst ip from StreamTable

			if stream, ok := StreamTable[dst]; ok {
				stream.Write(frame)
			} else {
				fmt.Println("ip:", dstStr)
				fmt.Println("dst:", dst)
				fmt.Println("stream not found")
			}
		}
	}()

	select {}
}
