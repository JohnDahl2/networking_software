package networkwatcher

import (
	"context"
	"live-sniffer/internal/ingestion"
	"live-sniffer/internal/storage"
	"log/slog"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcap"
	"github.com/jackc/pgx/v5/pgtype"
)

func OpenListenerOnNetwork(iface string) (*gopacket.PacketSource, error){
	handle, err := pcap.OpenLive(iface, 65535, true, pcap.BlockForever)
	if err != nil{
		return nil, err
	}
	return gopacket.NewPacketSource(handle, handle.LinkType()), nil
}


func ListenOnNetwork(ctx context.Context, packetSource *gopacket.PacketSource, sessionID pgtype.UUID, packetStream chan<- []storage.PacketRow, batchSize int) {
    batch := make([]storage.PacketRow, 0, batchSize)
    
    for packet := range packetSource.Packets() {
        select {
        case <-ctx.Done():
            if len(batch) > 0 {
                packetStream <- batch
            }
            slog.Warn("listening pipeline context cancelled; stopping listening thread")
            return
        default:
        }
        
        row := ingestion.PacketDecoder(sessionID, packet)
        batch = append(batch, row)
        
        if len(batch) >= batchSize {
            packetStream <- batch
            batch = make([]storage.PacketRow, 0, batchSize)
        }
    }
}