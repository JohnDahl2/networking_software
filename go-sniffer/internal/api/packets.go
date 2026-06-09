package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

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
	TCPFlags  *int      `json:"tcp_flags,omitempty"`
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

func resolveColumns(columnSlice []string) ([]string, error) {
	if len(columnSlice) == 0 {
		return defaultColumns, nil
	}
	validatedFields := []string{}
	for _, f := range columnSlice {
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
var ErrInvalidLimit = errors.New("invalid limit")
var ErrLimitOutOfRange = errors.New("limit out of range")
var ErrInvalidField         = errors.New("invalid field") 
var ErrCursor = errors.New("invalid cursor")
var ErrOrder  = errors.New("invalid order")
var ErrFilter = errors.New("invalid filter")

type ListPacketsParams struct {
    Limit   int
    Columns []string
    Cursor  *time.Time  // nil = no cursor
    Order   string
    Filters []Filter
}

type columnMapper struct {
    scan  func(r *storage.PacketRow) any
    mapTo func(r *storage.PacketRow, p *Packet)
}

func parseListPacketsParams(q url.Values) (ListPacketsParams, error){
	var columnSlice []string
	var cursor *time.Time
	limitStr := q.Get("limit")
	if limitStr == "" {
		limitStr = "100"
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		return ListPacketsParams{}, fmt.Errorf("%w: %w", ErrInvalidLimit, err)
	}
	if limit > 100 || limit < 1 {
		return ListPacketsParams{}, ErrLimitOutOfRange
	}
	if raw := q.Get("fields"); raw != "" {
		columnSlice = strings.Split(raw, ",")
	}
	columns, err := resolveColumns(columnSlice)
	if err != nil {
		return ListPacketsParams{}, fmt.Errorf("%w: %w", ErrInvalidField, err)
	}
	if cursorStr := q.Get("cursor"); cursorStr != "" {
		t, err := time.Parse(time.RFC3339, cursorStr)
		if err != nil {
			return ListPacketsParams{}, fmt.Errorf("%w: %w", ErrCursor, err)
		}
		cursor = &t
	}
	order, err := resolveOrder(q.Get("order"))
	if err != nil {
		return ListPacketsParams{}, fmt.Errorf("%w: %w", ErrOrder, err)
	}
	filters, err := resolveFilters(q["filter"])
	if err != nil {
		return ListPacketsParams{}, fmt.Errorf("%w: %w", ErrFilter, err)
	}
	return ListPacketsParams{
		Limit:   limit,
		Columns: columns,
		Cursor:  cursor,
		Order:   order,
		Filters: filters,
	}, nil
}

func buildPacketQuery(q ListPacketsParams)(query string, outArgs []any){
	args := make([]any, 0, len(q.Filters)+2)
	whereClauses := make([]string, 0, len(q.Filters)+1)
	argIdx := 1

	// If cursor is present it becomes the first WHERE clause and first arg.
	if q.Cursor != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("time > $%d", argIdx))
		args = append(args, q.Cursor)
		argIdx++
	}

	// Each filter adds a WHERE clause and a new arg.
	for _, f := range q.Filters {
		whereClauses = append(whereClauses, fmt.Sprintf("%s %s $%d", f.Field, f.Operator, argIdx))
		args = append(args, f.Value)
		argIdx++
	}

	// LIMIT is always the last arg.
	args = append(args, q.Limit)

	columnString := strings.Join(q.Columns, ", ")
	if len(whereClauses) > 0 {
		query = fmt.Sprintf(
			"SELECT %s FROM packet_logs WHERE %s ORDER BY time %s LIMIT $%d",
			columnString,
			strings.Join(whereClauses, " AND "),
			q.Order,
			argIdx,
		)
	} else {
		query = fmt.Sprintf(
			"SELECT %s FROM packet_logs ORDER BY time %s LIMIT $%d",
			columnString,
			q.Order,
			argIdx,
		)
	}
	return query, args
}

var columnMappers = map[string]columnMapper{
	"time": {
		scan:  func(r *storage.PacketRow) any { return &r.Time },
		mapTo: func(r *storage.PacketRow, p *Packet) { p.Timestamp = r.Time },
	},
	"src_ip": {
		scan:  func(r *storage.PacketRow) any { return &r.SrcIP },
		mapTo: func(r *storage.PacketRow, p *Packet) { s := r.SrcIP.String(); p.SrcIP = &s },
	},
	"dst_ip": {
		scan:  func(r *storage.PacketRow) any { return &r.DstIP },
		mapTo: func(r *storage.PacketRow, p *Packet) { s := r.DstIP.String(); p.DstIP = &s },
	},
	"src_port": {
		scan:  func(r *storage.PacketRow) any { return &r.SrcPort },
		mapTo: func(r *storage.PacketRow, p *Packet) { v := int(r.SrcPort); p.SrcPort = &v },
	},
	"dst_port": {
		scan:  func(r *storage.PacketRow) any { return &r.DstPort },
		mapTo: func(r *storage.PacketRow, p *Packet) { v := int(r.DstPort); p.DstPort = &v },
	},
	"protocol": {
		scan:  func(r *storage.PacketRow) any { return &r.Protocol },
		mapTo: func(r *storage.PacketRow, p *Packet) { p.Protocol = &r.Protocol },
	},
	"length": {
		scan:  func(r *storage.PacketRow) any { return &r.Length },
		mapTo: func(r *storage.PacketRow, p *Packet) { v := int(r.Length); p.Length = &v },
	},
	"tcp_flags": {
		scan:  func(r *storage.PacketRow) any { return &r.TCPFlags },
		mapTo: func(r *storage.PacketRow, p *Packet) { v := int(r.TCPFlags); p.TCPFlags = &v },
	},
	"job_id": {
		scan: func(r *storage.PacketRow) any { return &r.JobID },
		mapTo: func(r *storage.PacketRow, p *Packet) {
			var sid string
			if r.JobID.Valid {
				sid = r.JobID.String()
			}
			p.JobID = &sid
		},
	},
}

func packetQueryDb(ctx context.Context, q ListPacketsParams, DB storage.DBStore, query string, args []any) (PaginationResponse, error) {
	var nextCursor *string
	rows, err := DB.Query(ctx, query, args...)
	if err != nil {
		return PaginationResponse{}, fmt.Errorf("querying packets: %w", err)
	}
	defer rows.Close()

	dataQueried := make([]Packet, 0, q.Limit)

	for rows.Next() {
		var row storage.PacketRow
		scanTargets := make([]any, len(q.Columns))
		for i, col := range q.Columns {
			scanTargets[i] = columnMappers[col].scan(&row)
		}
		if err := rows.Scan(scanTargets...); err != nil {
			return PaginationResponse{}, fmt.Errorf("scanning row: %w", err)
		}
		p := Packet{}
		for _, col := range q.Columns {
			columnMappers[col].mapTo(&row, &p)
		}
		dataQueried = append(dataQueried, p)
	}
	if err := rows.Err(); err != nil {
		return PaginationResponse{}, fmt.Errorf("row iteration: %w", err)
	}

	if len(dataQueried) == q.Limit {
		t := dataQueried[len(dataQueried)-1].Timestamp.Format(time.RFC3339)
		nextCursor = &t
	}

	return PaginationResponse{Data: dataQueried, NextCursor: nextCursor}, nil
}

func (s *Server) HandleListPackets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	packetList, err := parseListPacketsParams(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	query, args := buildPacketQuery(packetList)
	paginatedData, err := packetQueryDb(r.Context(), packetList, s.DB, query, args)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse, err := json.Marshal(paginatedData)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(jsonResponse); err != nil {
		slog.Error("failed to write packets response", "error", err)
	}
}
