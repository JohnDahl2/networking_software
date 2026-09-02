package networkwatcher

import (
	"context"
	"encoding/json"
	"live-sniffer/internal/ingestion"
	"live-sniffer/internal/storage"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
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

func sendBatch(producer *kafka.Producer, topic string, batch []storage.PacketRow) error {
    mar, err := json.Marshal(batch)
    if err != nil {
        return err
    }
    return producer.Produce(&kafka.Message{
        TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
        Value:          mar,
    }, nil)
}


func ListenOnNetwork(ctx context.Context, packetSource *gopacket.PacketSource, sessionID pgtype.UUID, producer *kafka.Producer, topic string, batchSize int) {
    batch := make([]storage.PacketRow, 0, batchSize)
    
    for packet := range packetSource.Packets() {
        select {
        case <-ctx.Done():
            if len(batch) > 0 {
                err := sendBatch(producer, topic, batch)
                if err != nil{
                    slog.Warn("listening pipeline context was cancelled but not all data was flushed.", "error", err)
                }
            }
            slog.Warn("listening pipeline context cancelled; stopping listening thread")
            return
        default:
        }
        
        row := ingestion.PacketDecoder(sessionID, packet)
        if !row.SrcIP.IsValid() || !row.DstIP.IsValid() {
            continue
        }
        batch = append(batch, row)
        
        if len(batch) >= batchSize {
            err := sendBatch(producer, topic, batch)
                if err != nil{
                    slog.Warn("data was not flushed", "error", err)
                }
            batch = make([]storage.PacketRow, 0, batchSize)
        }
    }
}