package ingestion

import (
	"log/slog"
	"net/netip"
	"networking_software/shared/storage"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/jackc/pgx/v5/pgtype"
)


func PacketDecoder(sessionId pgtype.UUID, packet gopacket.Packet)storage.PacketRow{
	var srcIP, dstIP netip.Addr
	var srcPort, dstPort int32
	var protocol string
	var tcpFlags int16

	if ipLayer := packet.Layer(layers.LayerTypeIPv4); ipLayer != nil {
		ip := ipLayer.(*layers.IPv4)
		protocol = ip.Protocol.String()

		// Convert the 4-byte array directly to a netip.Addr (Zero Allocations!)
		srcIP = netip.AddrFrom4([4]byte(ip.SrcIP))
		dstIP = netip.AddrFrom4([4]byte(ip.DstIP))

	} else if ipv6Layer := packet.Layer(layers.LayerTypeIPv6); ipv6Layer != nil {
		ip := ipv6Layer.(*layers.IPv6)
		protocol = ip.NextHeader.String()

		// Convert the 16-byte array directly to a netip.Addr
		srcIP = netip.AddrFrom16([16]byte(ip.SrcIP))
		dstIP = netip.AddrFrom16([16]byte(ip.DstIP))
	} else {
		// DIAGNOSTIC NOTE: If packets are still hitting NULL, this will tell us what layer
		// Non-IP packet; skip it.
		slog.Debug("packet missing expected network layer", "layers", packet.Layers())
	}

	// Extract transport layer (TCP or UDP)
	if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
		tcp, _ := tcpLayer.(*layers.TCP)
		srcPort = int32(tcp.SrcPort)
		dstPort = int32(tcp.DstPort)

		// Read the raw TCP flags byte mask directly (SYN, ACK, FIN, etc.)
		tcpFlags = int16(packetFlagsToUint8(tcp))
	} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
		udp, _ := udpLayer.(*layers.UDP)
		srcPort = int32(udp.SrcPort)
		dstPort = int32(udp.DstPort)
	}
	

	row := storage.PacketRow{
		Time:     packet.Metadata().CaptureInfo.Timestamp,
		SrcIP:    srcIP,
		DstIP:    dstIP,
		SrcPort:  srcPort,
		DstPort:  dstPort,
		Protocol: protocol,
		Length:   int32(packet.Metadata().CaptureInfo.Length),
		TCPFlags: tcpFlags,
		JobID:    sessionId,
	}
	return row
}

func packetFlagsToUint8(tcp *layers.TCP) uint8 {
	var mask uint8
	if tcp.SYN {
		mask |= 1 << 1
	}
	if tcp.ACK {
		mask |= 1 << 4
	}
	if tcp.FIN {
		mask |= 1 << 0
	}
	if tcp.RST {
		mask |= 1 << 2
	}
	if tcp.PSH {
		mask |= 1 << 3
	}
	if tcp.URG {
		mask |= 1 << 5
	}
	return mask
}