package storage

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type Store struct{
	DB DBStore
}


func (s *Store) StartSession(ctx context.Context, iface string) (pgtype.UUID, error) {
    // Try to find an existing stopped/crashed session for this interface
    var sessionID pgtype.UUID
    err := s.DB.QueryRow(ctx, `
        UPDATE capture_sessions
        SET status = $1, started_at = $2, ended_at = NULL
        WHERE interface = $3
        AND status IN ('stopped', 'crashed')
        RETURNING session_id
    `, StatusStart, time.Now(), iface).Scan(&sessionID)

    if err == nil {
		return sessionID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		slog.Error("failed to update existing session", "error", err)
		return pgtype.UUID{}, err
	}

    err = s.DB.QueryRow(ctx, `
        INSERT INTO capture_sessions (interface, started_at, status)
        VALUES ($1, $2, $3)
        RETURNING session_id
    `, iface, time.Now(), StatusStart).Scan(&sessionID)

    if err != nil {
        slog.Error("failed to create session", "error", err)
        return pgtype.UUID{}, err
    }
    return sessionID, nil
}


func (s *Store) UpdateSession(ctx context.Context, sessionID pgtype.UUID, update string) {
    query := `
        UPDATE capture_sessions
        SET status = $1 WHERE session_id = $2
    `
    _, err := s.DB.Exec(ctx, query, update, sessionID)
    if err != nil {
        slog.Error("failed to update session", "error", err, "session_id", sessionID, "update", update)
    }
}


func (s *Store) GetSessionStatus(ctx context.Context, sessionID pgtype.UUID) (string, error) {
	var status string
	err := s.DB.QueryRow(ctx, `
		SELECT status FROM capture_sessions WHERE session_id = $1
	`, sessionID).Scan(&status)
	if err != nil {
		slog.Error("failed to get session status", "error", err, "session_id", sessionID)
		return "", err
	}
	return status, nil
}

func (s *Store)GetSessions(ctx context.Context) (pgx.Rows, error){
	query := `
		SELECT session_id, interface, started_at, ended_at, status, packets_saved
		FROM capture_sessions
		ORDER BY started_at DESC
	`
	data, err := s.DB.Query(ctx, query)
	if err != nil {
		slog.Error("failed to get sessions", "error", err)
		return nil, err
	}
	return data, nil
}

