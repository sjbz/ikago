package pcap

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"net"
)

// CreateTCPLayer returns a TCP layer.
func CreateTCPLayer(srcPort, dstPort uint16, seq, ack uint32) *layers.TCP {
	return &layers.TCP{
		SrcPort:    layers.TCPPort(srcPort),
		DstPort:    layers.TCPPort(dstPort),
		Seq:        seq,
		Ack:        ack,
		DataOffset: 5,
		PSH:        true,
		ACK:        true,
		Window:     65535,
		// Checksum: 0,
	}
}

// FlagTCPLayer reflags flags in a TCP layer.
func FlagTCPLayer(layer *layers.TCP, syn, psh, ack bool) {
	layer.SYN = syn
	layer.PSH = psh
	layer.ACK = ack
}

// CreateUDPLayer returns an UDP layer.
func CreateUDPLayer(srcPort, dstPort uint16) *layers.UDP {
	return &layers.UDP{
		SrcPort: layers.UDPPort(srcPort),
		DstPort: layers.UDPPort(dstPort),
		// Length:   0,
		// Checksum: 0,
	}
}

// CreateIPv4Layer returns an IPv4 layer.
func CreateIPv4Layer(srcIP, dstIP net.IP, id uint16, ttl uint8, transportLayer gopacket.TransportLayer) (*layers.IPv4, error) {
	if srcIP.To4() == nil {
		return nil, fmt.Errorf("invalid source ip %s", srcIP)
	}
	if dstIP.To4() == nil {
		return nil, fmt.Errorf("invalid destination ip %s", dstIP)
	}

	ipv4Layer := &layers.IPv4{
		Version: 4,
		IHL:     5,
		// Length: 0,
		Id:    id,
		Flags: layers.IPv4DontFragment,
		TTL:   ttl,
		// Protocol: 0,
		// Checksum: 0,
		SrcIP: srcIP,
		DstIP: dstIP,
	}

	// Protocol
	switch t := transportLayer.LayerType(); t {
	case layers.LayerTypeTCP:
		ipv4Layer.Protocol = layers.IPProtocolTCP

		// Checksum of transport layer
		tcpLayer := transportLayer.(*layers.TCP)
		err := tcpLayer.SetNetworkLayerForChecksum(ipv4Layer)
		if err != nil {
			return nil, fmt.Errorf("set network layer for checksum: %w", err)
		}
	case layers.LayerTypeUDP:
		ipv4Layer.Protocol = layers.IPProtocolUDP

		// Checksum of transport layer
		udpLayer := transportLayer.(*layers.UDP)
		err := udpLayer.SetNetworkLayerForChecksum(ipv4Layer)
		if err != nil {
			return nil, fmt.Errorf("set network layer for checksum: %w", err)
		}
	default:
		return nil, fmt.Errorf("transport layer type %s not support", t)
	}

	return ipv4Layer, nil
}

// FlagIPv4Layer reflags flags in an IPv4 layer.
func FlagIPv4Layer(layer *layers.IPv4, df, mf bool, offset uint16) {
	if df {
		layer.Flags = layers.IPv4DontFragment
	}
	if mf {
		layer.Flags = layers.IPv4MoreFragments
	}
	if !df && !mf {
		layer.Flags = 0
	}

	layer.FragOffset = offset
}

// CreateIPv6Layer returns an IPv6 layer.
func CreateIPv6Layer(srcIP, dstIP net.IP, hopLimit uint8, transportLayer gopacket.TransportLayer) (*layers.IPv6, error) {
	if srcIP.To4() != nil {
		return nil, fmt.Errorf("invalid source ip %s", srcIP)
	}
	if dstIP.To4() != nil {
		return nil, fmt.Errorf("invalid destination ip %s", dstIP)
	}

	ipv6Layer := &layers.IPv6{
		Version: 6,
		// Length: 0,
		HopLimit: hopLimit,
		SrcIP:    srcIP,
		DstIP:    dstIP,
	}

	// Protocol
	switch t := transportLayer.LayerType(); t {
	case layers.LayerTypeTCP:
		ipv6Layer.NextHeader = layers.IPProtocolTCP
	case layers.LayerTypeUDP:
		ipv6Layer.NextHeader = layers.IPProtocolUDP
	case layers.LayerTypeICMPv4:
		ipv6Layer.NextHeader = layers.IPProtocolICMPv4
	default:
		return nil, fmt.Errorf("transport layer type %s not support", t)
	}

	return ipv6Layer, nil
}

// CreateLoopbackLayer returns a loopback layer.
func CreateLoopbackLayer() *layers.Loopback {
	return &layers.Loopback{}
}

// CreateEthernetLayer returns an Ethernet layer.
func CreateEthernetLayer(srcMAC, dstMAC net.HardwareAddr, networkLayer gopacket.NetworkLayer) (*layers.Ethernet, error) {
	ethernetLayer := &layers.Ethernet{
		SrcMAC: srcMAC,
		DstMAC: dstMAC,
		// EthernetType: 0,
	}

	// Protocol
	switch t := networkLayer.LayerType(); t {
	case layers.LayerTypeIPv4:
		ethernetLayer.EthernetType = layers.EthernetTypeIPv4
	case layers.LayerTypeIPv6:
		ethernetLayer.EthernetType = layers.EthernetTypeIPv6
	default:
		return nil, fmt.Errorf("network layer type %s not support", t)
	}

	return ethernetLayer, nil
}

// Serialize serializes layers to byte array.
func Serialize(layers ...gopacket.SerializableLayer) ([]byte, error) {
	// Recalculate checksum and length
	options := gopacket.SerializeOptions{ComputeChecksums: true, FixLengths: true}
	buffer := gopacket.NewSerializeBuffer()

	err := gopacket.SerializeLayers(buffer, options, layers...)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// SerializeRaw serializes layers to byte array without computing checksums and updating lengths.
func SerializeRaw(layers ...gopacket.SerializableLayer) ([]byte, error) {
	// Recalculate checksum and length
	options := gopacket.SerializeOptions{}
	buffer := gopacket.NewSerializeBuffer()

	err := gopacket.SerializeLayers(buffer, options, layers...)
	if err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// CreateLayers return layers of transmission between client and server.
func CreateLayers(srcPort, dstPort uint16, seq, ack uint32, conn *RawConn, dstIP net.IP, id uint16, hop uint8,
	dstHardwareAddr net.HardwareAddr) (transportLayer, networkLayer, linkLayer gopacket.SerializableLayer, err error) {
	var (
		networkLayerType gopacket.LayerType
		linkLayerType    gopacket.LayerType
	)

	// Create transport layer
	transportLayer = CreateTCPLayer(srcPort, dstPort, seq, ack)

	// Decide IPv4 or IPv6
	if dstIP.To4() != nil {
		networkLayerType = layers.LayerTypeIPv4
	} else {
		networkLayerType = layers.LayerTypeIPv6
	}

	// Create new network layer
	switch networkLayerType {
	case layers.LayerTypeIPv4:
		networkLayer, err = CreateIPv4Layer(conn.LocalDev().IPv4Addr().IP, dstIP, id, hop-1, transportLayer.(gopacket.TransportLayer))
	case layers.LayerTypeIPv6:
		networkLayer, err = CreateIPv6Layer(conn.LocalDev().IPv6Addr().IP, dstIP, hop-1, transportLayer.(gopacket.TransportLayer))
	default:
		return nil, nil, nil, fmt.Errorf("network layer type %s not support", networkLayerType)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create network layer: %w", err)
	}

	// Decide Loopback or Ethernet
	if conn.IsLoop() {
		linkLayerType = layers.LayerTypeLoopback
	} else {
		linkLayerType = layers.LayerTypeEthernet
	}

	// Create new link layer
	switch linkLayerType {
	case layers.LayerTypeLoopback:
		linkLayer = CreateLoopbackLayer()
	case layers.LayerTypeEthernet:
		linkLayer, err = CreateEthernetLayer(conn.LocalDev().HardwareAddr(), dstHardwareAddr, networkLayer.(gopacket.NetworkLayer))
	default:
		return nil, nil, nil, fmt.Errorf("link layer type %s not support", linkLayerType)
	}
	if err != nil {
		return nil, nil, nil, fmt.Errorf("create link layer: %w", err)
	}

	return transportLayer, networkLayer, linkLayer, nil
}
