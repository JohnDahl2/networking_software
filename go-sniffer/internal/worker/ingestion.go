package worker

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/netip"
	"os"
	"strings"
	"sync"
	"sync/atomic"

	"go-sniffer/internal/storage"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcapgo"
	"github.com/jackc/pgx/v5/pgtype"
)

func PcapWorker(ctx context.Context, DB storage.DBStore, jobID pgtype.UUID, totalRead *int64, id int, jobs <-chan string, packetStream chan<- []storage.PacketRow, wg *sync.WaitGroup) {
	defer wg.Done()
	log := slog.With("reader_id", id)

	for path := range jobs {
		select {
		case <-ctx.Done():
			log.Warn("pipeline context cancelled; stopping reader thread early")
			return
		default:
		}

		const batchLimit = 500

		err := func(p string) error {
			log.Debug("starting file extraction sequence", "file_path", p)

			f, err := os.Open(p)
			if err != nil {
				return err
			}
			defer f.Close()

			bufferedReader := bufio.NewReader(f)
			reader, err := pcapgo.NewReader(bufferedReader)
			if err != nil {
				return err
			}

			currentBatch := make([]storage.PacketRow, 0, batchLimit)

			for {
				if len(currentBatch) == 0 {
					select {
					case <-ctx.Done():
						return ctx.Err()
					default:
					}
				}

				data, captureInfo, err := reader.ReadPacketData()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					// If the error message contains "EOF", it means we safely hit the end of the file
					// structure but the reader loop just wants to exit. Break out cleanly!
					if strings.Contains(err.Error(), "EOF") {
						break
					}

					// Real structural corruptions will still surface here cleanly
					log.Debug("skipping corrupted packet within valid file structure", "file_path", p, "error", err.Error())
					continue
				}
				packet := gopacket.NewPacket(data, reader.LinkType(), gopacket.Default)
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
					Time:     captureInfo.Timestamp, // Grab the actual time the packet hit the wire!
					SrcIP:    srcIP,
					DstIP:    dstIP,
					SrcPort:  srcPort,
					DstPort:  dstPort,
					Protocol: protocol,
					Length:   int32(captureInfo.Length),
					TCPFlags: tcpFlags,
					JobID:    jobID,
				}

				currentBatch = append(currentBatch, row)
				if len(currentBatch) >= batchLimit {
					select {
					case packetStream <- currentBatch:
					case <-ctx.Done():
						return ctx.Err()
					}
					atomic.AddInt64(totalRead, int64(len(currentBatch)))
					currentBatch = make([]storage.PacketRow, 0, batchLimit)
				}
			}

			if len(currentBatch) > 0 {
				select {
				case packetStream <- currentBatch:
				case <-ctx.Done():
					return ctx.Err()
				}
				atomic.AddInt64(totalRead, int64(len(currentBatch)))
			}

			return nil
		}(path)

		if err == nil {
			storage.UpdateJobProgress(ctx, DB, jobID, 1)
		}

		// Handle errors from the file closure.
		if err != nil {
			// If the error was just a deliberate context cancellation, log it as a quiet note, not a massive error failure
			if ctx.Err() != nil {
				log.Debug("file extraction aborted cleanly by application context shutdown", "file_path", path)
			} else {
				log.Error("failed to process target pcap file completely", "file_path", path, "error", err.Error())
			}
		}
	}
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
