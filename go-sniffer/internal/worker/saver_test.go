package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

	"go-sniffer/internal/storage"
)

// mockSaverDB lets tests control CopyFrom and Exec independently.
type mockSaverDB struct {
	copyFromFn func(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error)
	execFn     func(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func (m *mockSaverDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}
func (m *mockSaverDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
func (m *mockSaverDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.execFn != nil {
		return m.execFn(ctx, sql, args...)
	}
	return pgconn.CommandTag{}, nil
}
func (m *mockSaverDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	if m.copyFromFn != nil {
		return m.copyFromFn(ctx, tableName, columnNames, rowSrc)
	}
	return 0, nil
}

var testJobID = pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

func makeBatch(n int) []storage.PacketRow {
	rows := make([]storage.PacketRow, n)
	for i := range rows {
		rows[i] = storage.PacketRow{Time: time.Now(), Protocol: "TCP"}
	}
	return rows
}

// runSaver wires up a PacketSaverWorker and returns the results channel.
// The caller controls packetStream.
func runSaver(ctx context.Context, db storage.DBStore, cancel context.CancelFunc, packetStream chan []storage.PacketRow) chan int {
	results := make(chan int, 1)
	var totalSaved int64
	go PacketSaverWorker(ctx, db, testJobID, &totalSaved, 1, packetStream, results, cancel)
	return results
}

func TestPacketSaverWorker_HappyPath(t *testing.T) {
	db := &mockSaverDB{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packetStream := make(chan []storage.PacketRow, 1)
	results := runSaver(ctx, db, cancel, packetStream)

	packetStream <- makeBatch(10)
	close(packetStream)

	saved := <-results
	if saved != 10 {
		t.Errorf("want 10 saved, got %d", saved)
	}
}

func TestPacketSaverWorker_DBWriteFailure(t *testing.T) {
	db := &mockSaverDB{
		copyFromFn: func(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			return 0, errors.New("disk full")
		},
	}
	ctx, cancel := context.WithCancel(context.Background())

	packetStream := make(chan []storage.PacketRow, 1)
	results := runSaver(ctx, db, cancel, packetStream)

	packetStream <- makeBatch(5)

	// Worker should cancel the context and exit after the DB error.
	select {
	case <-results:
		// good — worker exited
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after DB failure")
	}

	// Context must have been cancelled.
	if ctx.Err() == nil {
		t.Error("expected context to be cancelled after DB failure")
	}
}

func TestPacketSaverWorker_ContextCancelled(t *testing.T) {
	db := &mockSaverDB{}
	ctx, cancel := context.WithCancel(context.Background())

	packetStream := make(chan []storage.PacketRow)
	results := runSaver(ctx, db, cancel, packetStream)

	cancel()

	select {
	case <-results:
		// good — worker exited cleanly
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not exit after context cancellation")
	}
}

func TestPacketSaverWorker_ChannelClosedSendsCount(t *testing.T) {
	db := &mockSaverDB{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packetStream := make(chan []storage.PacketRow, 2)
	results := runSaver(ctx, db, cancel, packetStream)

	packetStream <- makeBatch(7)
	packetStream <- makeBatch(3)
	close(packetStream)

	saved := <-results
	if saved != 10 {
		t.Errorf("want 10 saved, got %d", saved)
	}
}

func TestPacketSaverWorker_MultipleBatchesAccumulate(t *testing.T) {
	writes := 0
	db := &mockSaverDB{
		copyFromFn: func(_ context.Context, _ pgx.Identifier, _ []string, _ pgx.CopyFromSource) (int64, error) {
			writes++
			return 0, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	packetStream := make(chan []storage.PacketRow, 3)
	results := runSaver(ctx, db, cancel, packetStream)

	packetStream <- makeBatch(100)
	packetStream <- makeBatch(200)
	packetStream <- makeBatch(300)
	close(packetStream)

	saved := <-results
	if saved != 600 {
		t.Errorf("want 600 saved, got %d", saved)
	}
	if writes != 3 {
		t.Errorf("want 3 DB writes, got %d", writes)
	}
}
