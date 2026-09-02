package pipeline

import (
	"context"
	"fmt"
	"live-sniffer/internal/network"
	"live-sniffer/internal/proto"
	"live-sniffer/internal/storage"
	"live-sniffer/internal/workers"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/jackc/pgx/v5/pgtype"
)

type Launcher struct {
	DB              storage.DBStore
	SaverWorkers    int
	SessionMu      sync.Mutex
	Sessions		map[string]context.CancelFunc

}

func (l *Launcher) StartPipeline(ctx context.Context, iface string, workerSaverCount int) (string, error){
	var sessionIDString string
	var err error
	if workerSaverCount < 1 || workerSaverCount > 10 {
        return sessionIDString, fmt.Errorf("worker count must be between 1 and 10, got %d", workerSaverCount)
    }

	var totalSaved int64

	kafkaURL := os.Getenv("KAFKA_URL")
	kafkaTopic := os.Getenv("KAFKA_TOPIC")
	
	producer, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": kafkaURL,
	})
	if err != nil {
		slog.Error("Issue with starting the producer", "error", err)
		return sessionIDString, err
	}

	ctx, cancel := context.WithCancel(ctx)

	store := &storage.Store{DB: l.DB}

	ps, err := networkwatcher.OpenListenerOnNetwork(iface)
	if err != nil{
		return sessionIDString, err
	}
	sessionID, err := store.StartSession(ctx, iface)
	if err != nil {
		return sessionIDString, err
	}
	sessionIDString = sessionID.String()
	l.SessionMu.Lock()
	l.Sessions[sessionIDString] = cancel
	l.SessionMu.Unlock()

	onFailure := func() {
		store.UpdateSession(ctx, sessionID, storage.StatusCrashed)
	}

	for w := 1; w <= workerSaverCount; w++ {
		consumer, err := kafka.NewConsumer(&kafka.ConfigMap{
			"bootstrap.servers": kafkaURL,
			"group.id": "packet-savers",
			"auto.offset.reset": "earliest",
		})
		if err != nil {
			return sessionIDString, err
		}
		consumer.SubscribeTopics([]string{kafkaTopic}, nil)
		go workers.PacketSaverWorker(ctx, l.DB, &totalSaved, w, consumer, cancel, onFailure)
	}

	go networkwatcher.ListenOnNetwork(ctx, ps, sessionID, producer, kafkaTopic, 500)
	store.UpdateSession(ctx, sessionID, storage.StatusRunning)

	return sessionIDString, nil
}

func (l *Launcher) StopPipeline(ctx context.Context, sessionIDString string) error{
	var sessionID pgtype.UUID
	err := sessionID.Scan(sessionIDString)
	if err != nil {
		return err
	}
	store := &storage.Store{DB: l.DB}
	l.SessionMu.Lock()
	if cancel, ok := l.Sessions[sessionIDString]; ok {
		cancel()
		delete(l.Sessions, sessionIDString)
	}
	l.SessionMu.Unlock()
	store.UpdateSession(ctx, sessionID, storage.StatusFinish)
	return nil
}

func (l *Launcher) StopAll(ctx context.Context) {
    l.SessionMu.Lock()
    ids := make([]string, 0, len(l.Sessions))
    for id := range l.Sessions {
        ids = append(ids, id)
    }
    l.SessionMu.Unlock()
    
    for _, id := range ids {
        l.StopPipeline(ctx, id)
    }
}

func (l *Launcher) ViewPipelines(ctx context.Context) ([]proto.Session, error) {
	var s []proto.Session
	store := &storage.Store{DB: l.DB}
	rows, err := store.GetSessions(ctx)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var sessionID pgtype.UUID
		var iface, status string
		var startedAt time.Time
		var endedAt *time.Time
		var packetsSaved int64

		t := proto.Session{}

		if err := rows.Scan(&sessionID, &iface, &startedAt, &endedAt, &status, &packetsSaved); err != nil {
			continue
		}
		if endedAt != nil {
			formatted := endedAt.Format(time.RFC3339)
			t.EndedAt = &formatted
		}
		t.ID = sessionID.String()
		t.Interface = iface
		t.StartedAt = startedAt.Format(time.RFC3339)
		t.Status = status
		t.PacketsSaved = packetsSaved
		s = append(s, t)

	}
	return s, rows.Err()
}
