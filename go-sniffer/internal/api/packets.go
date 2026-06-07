package api

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"go-sniffer/internal/storage"
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
	"time":      true,
	"src_ip":    true,
	"dst_ip":    true,
	"src_port":  true,
	"dst_port":  true,
	"protocol":  true,
	"length":    true,
	"tcp_flags": true,
	"job_id": true,
}

var validExpressions = map[string]string{
	"eq":  "=",
	"ne":  "!=",
	"gt":  ">",
	"lt":  "<",
	"gte": ">=",
	"lte": "<=",
}

type Packet struct {
	Timestamp time.Time `json:"time"`
	SrcIP     *string   `json:"src_ip,omitempty"`
	DstIP     *string   `json:"dst_ip,omitempty"`
	SrcPort   *int      `json:"src_port,omitempty"`
	DstPort   *int      `json:"dst_port,omitempty"`
	Protocol  *string   `json:"protocol,omitempty"`
	Length    *int      `json:"length,omitempty"`
	TcpFlags  *int      `json:"tcp_flags,omitempty"`
	JobID  *string   `json:"job_id,omitempty"`
}

type PaginationResponse struct {
	Data       []Packet `json:"data"`
	NextCursor *string  `json:"next_cursor,omitempty"`
}

// Filter holds a parsed and validated filter expression.
type Filter struct {
	Field    string
	Operator string // SQL operator e.g. "=", ">", "<="
	Value    string
}

func resolveOrder(orderstring string) (string, error) {
	if orderstring == "desc" {
		return "DESC", nil
	} else if orderstring == "" || orderstring == "asc" {
		return "ASC", nil
	}
	return "", fmt.Errorf("unknown order value: %q, use asc or desc", orderstring)
}

func resolveColumns(columSlice []string) ([]string, error) {
	if len(columSlice) == 0 {
		return defaultColumns, nil
	}
	validatedFields := []string{}
	for _, f := range columSlice {
		if !validFields[f] {
			return nil, fmt.Errorf("unknown field: %q", f)
		}
		validatedFields = append(validatedFields, f)
	}
	return validatedFields, nil
}

// resolveFilters parses each "field:op:value" string and validates the field and operator.
func resolveFilters(filterSlice []string) ([]Filter, error) {
	filters := make([]Filter, 0, len(filterSlice))
	for _, f := range filterSlice {
		parts := strings.SplitN(f, ":", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("invalid filter format %q, expected field:op:value", f)
		}
		field, op, value := parts[0], parts[1], parts[2]

		if !validFields[field] {
			return nil, fmt.Errorf("unknown filter field: %q", field)
		}
		sqlOp, ok := validExpressions[op]
		if !ok {
			return nil, fmt.Errorf("unknown filter operator: %q", op)
		}
		filters = append(filters, Filter{Field: field, Operator: sqlOp, Value: value})
	}
	return filters, nil
}

func (s *Server) HandleListPackets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var cursor *time.Time
	var query string
	var order string
	var rows pgx.Rows
	var nextCursor *string

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
	if limit > 100 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`Upper limit is 100 for the time being`))
		return
	}

	// Parse and validate fields
	var columnSlice []string
	if raw := q.Get("fields"); raw != "" {
		columnSlice = strings.Split(raw, ",")
	}
	columns, err := resolveColumns(columnSlice)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("There was an issue with your field err: %v", err)))
		return
	}

	// Parse and validate cursor
	cursorStr := q.Get("cursor")
	if cursorStr != "" {
		parsedTime, err := time.Parse(time.RFC3339, cursorStr)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(fmt.Sprintf("There was an issue with the pagination: %v", err)))
			return
		}
		cursor = &parsedTime
	}

	// Parse and validate order
	order, err = resolveOrder(q.Get("order"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("There was an issue with the order: %v", err)))
		return
	}

	// Parse and validate filters
	filters, err := resolveFilters(q["filter"])
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("There was an issue with a filter: %v", err)))
		return
	}

	// Build the query dynamically.
	// args holds the parameterized values in order, argIdx tracks the $N position.
	args := []any{}
	argIdx := 1
	whereClauses := []string{}

	// If cursor is present it becomes the first WHERE clause and first arg.
	if cursor != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("time > $%d", argIdx))
		args = append(args, cursor)
		argIdx++
	}

	// Each filter adds a WHERE clause and a new arg.
	for _, f := range filters {
		whereClauses = append(whereClauses, fmt.Sprintf("%s %s $%d", f.Field, f.Operator, argIdx))
		args = append(args, f.Value)
		argIdx++
	}

	// LIMIT is always the last arg.
	args = append(args, limit)

	columnString := strings.Join(columns, ", ")
	if len(whereClauses) > 0 {
		query = fmt.Sprintf(
			"SELECT %s FROM packet_logs WHERE %s ORDER BY time %s LIMIT $%d",
			columnString,
			strings.Join(whereClauses, " AND "),
			order,
			argIdx,
		)
	} else {
		query = fmt.Sprintf(
			"SELECT %s FROM packet_logs ORDER BY time %s LIMIT $%d",
			columnString,
			order,
			argIdx,
		)
	}

	rows, err = s.DB.Query(r.Context(), query, args...)
	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte(fmt.Sprintf("DB Error: %v", err)))
		return
	}
	defer rows.Close()

	dataQueried := make([]Packet, 0, limit)

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
			case "job_id":
				scanTargets[i] = &row.JobID
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
			case "job_id":
				var sid string
				if row.JobID.Valid {
					sid = row.JobID.String()
				}
				p.JobID = &sid
			}
		}
		dataQueried = append(dataQueried, p)
	}

	if len(dataQueried) == limit {
		t := dataQueried[len(dataQueried)-1].Timestamp.Format(time.RFC3339)
		nextCursor = &t
	} else {
		nextCursor = nil
	}

	paginatedData := PaginationResponse{Data: dataQueried, NextCursor: nextCursor}
	jsonResponse, err := json.Marshal(paginatedData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}
