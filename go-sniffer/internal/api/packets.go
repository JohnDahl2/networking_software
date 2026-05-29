package api

import (
	"strings"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"fmt"

	"go-sniffer/internal/storage"
	"time"
)

var defaultColumns = []string{
	"time", 
	"src_ip", 
	"dst_ip", 
	"src_port", 
	"dst_port", 
	"protocol", 
	"length", 
	"tcp_flags",
}
var validFields = map[string]bool{
	"time":       true,
	"src_ip":     true,
	"dst_ip":     true,
	"src_port":   true,
	"dst_port":   true,
	"protocol":   true,
	"length":     true,
	"tcp_flags":  true,
	"stream_id":  true, 
}

type Packet struct {
	Timestamp time.Time `json:"time"`
	SrcIP     *string    `json:"src_ip,omitempty"`
	DstIP     *string    `json:"dst_ip,omitempty"`
	SrcPort   *int       `json:"src_port,omitempty"`
	DstPort   *int       `json:"dst_port,omitempty"`
	Protocol  *string    `json:"protocol,omitempty"`
	Length    *int       `json:"length,omitempty"`
	TcpFlags  *int       `json:"tcp_flags,omitempty"`
	StreamID  *string    `json:"stream_id,omitempty"`
}

func resolveColumns(columString string) ([]string,error) {
	if columString == "" {
		return defaultColumns, nil
	}
	stringSlice := strings.Split(columString, ",")
	validatedFields := []string{}
	for _,f :=range(stringSlice) {
		if !validFields[f] {
			return nil, fmt.Errorf("unknown field: %q", f)
		}
		validatedFields = append(validatedFields, f)
	}
	return validatedFields, nil
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

	columnFields := q.Get("fields")
	columns, err := resolveColumns(columnFields)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf(`There was an issue with your field err: %v`, err)))
		return
	}

	columnString := strings.Join(columns, ", ")
	query := fmt.Sprintf("SELECT %s FROM packet_logs LIMIT $1", columnString)
	rows, err := s.DB.Query(r.Context(), query, limit)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
        w.Write([]byte(fmt.Sprintf("DB Error: %v", err)))
		return
	}
	dataQueried := make([]Packet, 0, limit)

	defer rows.Close()
	for rows.Next() {
		var row storage.PacketRow
		scanTargets := make([]any, len(columns))
		for i, col := range columns {
			switch col {
			case "time":
				scanTargets[i] = &row.Time
			case "src_ip":
				scanTargets[i] = &row.SrcIP
			case "dst_ip":
				scanTargets[i] = &row.DstIP
			case "src_port":
				scanTargets[i] = &row.SrcPort
			case "dst_port":
				scanTargets[i] = &row.DstPort
			case "protocol":
				scanTargets[i] = &row.Protocol
			case "length":
				scanTargets[i] = &row.Length
			case "tcp_flags":
				scanTargets[i] = &row.TCPFlags
			case "stream_id":
				scanTargets[i] = &row.StreamID
			}
		}

		if err := rows.Scan(scanTargets...); err != nil {
			slog.Error("database row scan failed", "error", err)
			http.Error(w, "error scanning database row", http.StatusBadGateway)
			return
		}

		p := Packet{}
		for _, col := range columns {
			switch col {
			case "time":
				p.Timestamp = row.Time
			case "src_ip":
				s := row.SrcIP.String()
				p.SrcIP = &s
			case "dst_ip":
				s := row.DstIP.String()
				p.DstIP = &s
			case "src_port":
				v := int(row.SrcPort)
				p.SrcPort = &v
			case "dst_port":
				v := int(row.DstPort)
				p.DstPort = &v
			case "protocol":
				p.Protocol = &row.Protocol
			case "length":
				v := int(row.Length)
				p.Length = &v
			case "tcp_flags":
				v := int(row.TCPFlags)
				p.TcpFlags = &v
			case "stream_id":
				var sid string
				if row.StreamID.Valid {
					sid = row.StreamID.String()
				}
				p.StreamID = &sid
			}
		}
		dataQueried = append(dataQueried, p)
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
