package pipeline

import (
	"context"
	"fmt"
	"live-sniffer/internal/network"
	"live-sniffer/internal/storage"
	"live-sniffer/internal/workers"
	"log/slog"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

type Launcher struct {
	DB              storage.DBStore
	SaverWorkers    int
	SessionMu      sync.Mutex
	Sessions		map[string]context.CancelFunc

}

func (l *Launcher) StartPipeline(ctx context.Context, iface string, workerSaverCount int) error{
	if workerSaverCount < 1 || workerSaverCount > 10 {
        return fmt.Errorf("worker count must be between 1 and 10, got %d", workerSaverCount)
    }

	var totalSaved int64
	finalCounts := make(chan int, workerSaverCount)
	packetStream := make(chan []storage.PacketRow, 100)

	ctx, cancel := context.WithCancel(ctx)

	store := &storage.Store{DB: l.DB}

	ps, err := networkwatcher.OpenListenerOnNetwork(iface)
	if err != nil{
		return err
	}
	sessionID, err := store.StartSession(ctx, iface)
	if err != nil {
		return err
	}
	l.SessionMu.Lock()
	l.Sessions[sessionID.String()] = cancel
	l.SessionMu.Unlock()

	onFailure := func() {
		store.UpdateSession(ctx, sessionID, storage.StatusCrashed) //something gone wrong
	}

	for w := 1; w <= workerSaverCount; w++ {
		go workers.PacketSaverWorker(ctx, l.DB, &totalSaved, w, packetStream, finalCounts, cancel, onFailure)
	}
	go networkwatcher.ListenOnNetwork(ctx, ps, sessionID, packetStream, 500)
	store.UpdateSession(ctx, sessionID, storage.StatusRunning)

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				status, err := store.GetSessionStatus(ctx, sessionID)
				if err != nil {
					slog.Error("failed to poll session status", "error", err)
					continue
				}
				if status == storage.StatusFinish {
					slog.Info("stop signal received, shutting down", "session_id", sessionID)
					cancel()
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
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

func (l *Launcher) ViewPipelines(ctx context.Context) error {
	store := &storage.Store{DB: l.DB}
	rows, err := store.GetSessions(ctx)
	if err != nil {
		return err
	}
	defer rows.Close()

	fmt.Printf("%-36s  %-10s  %-20s  %-14s  %s\n", "SESSION ID", "INTERFACE", "STARTED AT", "PACKETS SAVED", "STATUS")
	fmt.Println("------------------------------------------------------------------------------------------------------------")
	for rows.Next() {
		var sessionID pgtype.UUID
		var iface, status string
		var startedAt time.Time
		var endedAt *time.Time
		var packetsSaved int64

		if err := rows.Scan(&sessionID, &iface, &startedAt, &endedAt, &status, &packetsSaved); err != nil {
			return err
		}
		fmt.Printf("%-36s  %-10s  %-20s  %-14d  %s\n",
			sessionID.String(),
			iface,
			startedAt.Format("2006-01-02 15:04:05"),
			packetsSaved,
			status,
		)
	}
	return rows.Err()
}
