package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"fmt"

	"go-sniffer/internal/storage"
	"time"
)

type Packet struct {
	Timestamp time.Time `json:"time"`
	SrcIP     string    `json:"src_ip"`
	DstIP     string    `json:"dst_ip"`
	Length    int       `json:"length"`
}

func (s *Server) HandleListPackets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Limit was not a number bad boy!`))
		return
	}

	if limit > 100{
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Upper limit is 100 for the time being`))
		return
	}
	dataQueried := make([]Packet, 0, limit)


	query := `SELECT time, src_ip, dst_ip, length FROM packet_logs LIMIT $1`
	rows, err := s.DB.Query(r.Context(), query, limit)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
        w.Write([]byte(fmt.Sprintf("DB Error: %v", err)))
		return
	}
	defer rows.Close()

	for rows.Next() {
		var row storage.PacketRow

		err := rows.Scan(&row.Time, &row.SrcIP, &row.DstIP, &row.Length)
		if err != nil {
			slog.Error("Database row scan failed", "error", err)
			w.WriteHeader(http.StatusBadGateway)
			w.Write([]byte(`Error scanning database row`))
			return
		}
		dataQueried = append(dataQueried, Packet{
					Timestamp: row.Time,
					SrcIP:     row.SrcIP.String(),
					DstIP:     row.DstIP.String(),
					Length:    int(row.Length),
				})
	}
	jsonResponse, err := json.Marshal(dataQueried)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}
