package api

import (
	"context"
	"errors"
	"net/netip"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"go-sniffer/internal/storage"
)


type mockDB struct {
	queryFn func(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func (m *mockDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return m.queryFn(ctx, sql, args...)
}
func (m *mockDB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}
func (m *mockDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}
func (m *mockDB) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return 0, nil
}

type mockRows struct {
	rows    []storage.PacketRow
	pos     int
	iterErr error
	scanFn  func(row storage.PacketRow, dest []any) error
}

func (m *mockRows) Next() bool {
	m.pos++
	return m.pos <= len(m.rows)
}

func (m *mockRows) Scan(dest ...any) error {
	return m.scanFn(m.rows[m.pos-1], dest)
}

func (m *mockRows) Err() error                                   { return m.iterErr }
func (m *mockRows) Close()                                       {}
func (m *mockRows) CommandTag() pgconn.CommandTag                { return pgconn.CommandTag{} }
func (m *mockRows) FieldDescriptions() []pgconn.FieldDescription { return nil }
func (m *mockRows) Values() ([]any, error)                       { return nil, nil }
func (m *mockRows) RawValues() [][]byte                          { return nil }
func (m *mockRows) Conn() *pgx.Conn                              { return nil }

// scanTimeProtocol is a scanFn for tests that use fields=["time","protocol"].
func scanTimeProtocol(row storage.PacketRow, dest []any) error {
	*dest[0].(*time.Time) = row.Time
	*dest[1].(*string) = row.Protocol
	return nil
}

// ---------------------------------------------------------------------------
// Existing tests
// ---------------------------------------------------------------------------

func TestResolveOrder(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		wantErr  bool
	}{
		{"asc", "ASC", false},
		{"desc", "DESC", false},
		{"", "ASC", false},
		{"bad", "", true},
		{"sideways", "", true},
	}

	for _, tc := range cases {
		result, err := resolveOrder(tc.input)
		if tc.wantErr && err == nil {
			t.Errorf("expected error for input %q but got none", tc.input)
		}
		if !tc.wantErr && result != tc.expected {
			t.Errorf("input %q: expected %q got %q", tc.input, tc.expected, result)
		}
	}
}

func TestResolveColumns(t *testing.T) {
	type testCase struct {
		name          string
		input         []string
		wantResult    []string
		wantErrSubstr string
	}

	tests := []testCase{
		{
			name:          "empty input returns default columns",
			input:         []string{},
			wantResult:    defaultColumns,
			wantErrSubstr: "",
		},
		{
			name:          "nil input returns default columns",
			input:         nil,
			wantResult:    defaultColumns,
			wantErrSubstr: "",
		},
		{
			name:          "valid fields are returned",
			input:         []string{"src_ip", "dst_ip", "length"},
			wantResult:    []string{"src_ip", "dst_ip", "length"},
			wantErrSubstr: "",
		},
		{
			name:          "single valid field",
			input:         []string{"protocol"},
			wantResult:    []string{"protocol"},
			wantErrSubstr: "",
		},
		{
			name:          "job_id is a valid field",
			input:         []string{"src_ip", "job_id"},
			wantResult:    []string{"src_ip", "job_id"},
			wantErrSubstr: "",
		},
		{
			name:          "invalid field triggers error",
			input:         []string{"src_ip", "password"},
			wantResult:    nil,
			wantErrSubstr: `unknown field: "password"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, err := resolveColumns(tc.input)

			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error %q but got nil", tc.wantErrSubstr)
				}
				if err.Error() != tc.wantErrSubstr {
					t.Errorf("got error %q, want %q", err.Error(), tc.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(gotResult, tc.wantResult) {
				t.Errorf("got %v, want %v", gotResult, tc.wantResult)
			}
		})
	}
}

func TestResolveFilters(t *testing.T) {
	type testCase struct {
		name          string
		input         []string
		wantResult    []Filter
		wantErrSubstr string
	}

	tests := []testCase{
		{
			name:  "valid filters are parsed correctly",
			input: []string{"protocol:eq:TCP", "length:gt:500"},
			wantResult: []Filter{
				{Field: "protocol", Operator: "=", Value: "TCP"},
				{Field: "length", Operator: ">", Value: "500"},
			},
			wantErrSubstr: "",
		},
		{
			name:  "all operators map correctly",
			input: []string{"length:eq:100", "length:ne:200", "length:gt:300", "length:lt:400", "length:gte:500", "length:lte:600"},
			wantResult: []Filter{
				{Field: "length", Operator: "=", Value: "100"},
				{Field: "length", Operator: "!=", Value: "200"},
				{Field: "length", Operator: ">", Value: "300"},
				{Field: "length", Operator: "<", Value: "400"},
				{Field: "length", Operator: ">=", Value: "500"},
				{Field: "length", Operator: "<=", Value: "600"},
			},
			wantErrSubstr: "",
		},
		{
			name:          "missing value part triggers error",
			input:         []string{"protocol:eq"},
			wantResult:    nil,
			wantErrSubstr: `invalid filter format "protocol:eq", expected field:op:value`,
		},
		{
			name:  "value with colon is preserved by SplitN",
			input: []string{"src_ip:eq:10.0.0.1:extra"},
			wantResult: []Filter{
				{Field: "src_ip", Operator: "=", Value: "10.0.0.1:extra"},
			},
			wantErrSubstr: "",
		},
		{
			name:          "unknown field triggers error",
			input:         []string{"password:eq:secret"},
			wantResult:    nil,
			wantErrSubstr: `unknown filter field: "password"`,
		},
		{
			name:          "unknown operator triggers error",
			input:         []string{"protocol:like:TCP"},
			wantResult:    nil,
			wantErrSubstr: `unknown filter operator: "like"`,
		},
		{
			name:          "empty input returns empty slice",
			input:         []string{},
			wantResult:    []Filter{},
			wantErrSubstr: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotResult, err := resolveFilters(tc.input)

			if tc.wantErrSubstr != "" {
				if err == nil {
					t.Fatalf("expected error %q but got nil", tc.wantErrSubstr)
				}
				if err.Error() != tc.wantErrSubstr {
					t.Errorf("got error %q, want %q", err.Error(), tc.wantErrSubstr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if !reflect.DeepEqual(gotResult, tc.wantResult) {
				t.Errorf("got %v, want %v", gotResult, tc.wantResult)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestParseListPacketsParams (item 12)
// ---------------------------------------------------------------------------

func TestParseListPacketsParams(t *testing.T) {
	t.Run("defaults when no params", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Limit != 100 {
			t.Errorf("want limit 100, got %d", p.Limit)
		}
		if !reflect.DeepEqual(p.Columns, defaultColumns) {
			t.Errorf("want default columns, got %v", p.Columns)
		}
		if p.Cursor != nil {
			t.Errorf("want nil cursor, got %v", p.Cursor)
		}
		if p.Order != "ASC" {
			t.Errorf("want ASC, got %s", p.Order)
		}
		if len(p.Filters) != 0 {
			t.Errorf("want no filters, got %v", p.Filters)
		}
	})

	t.Run("custom limit", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{"limit": {"50"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Limit != 50 {
			t.Errorf("want 50, got %d", p.Limit)
		}
	})

	t.Run("limit over 100 rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"limit": {"101"}})
		if !errors.Is(err, ErrInvalidLimitGreaterLess) {
			t.Errorf("want ErrInvalidLimitGreaterLess, got %v", err)
		}
	})

	t.Run("limit of 0 rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"limit": {"0"}})
		if !errors.Is(err, ErrInvalidLimitGreaterLess) {
			t.Errorf("want ErrInvalidLimitGreaterLess, got %v", err)
		}
	})

	t.Run("non-numeric limit rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"limit": {"abc"}})
		if !errors.Is(err, ErrInvalidLimit) {
			t.Errorf("want ErrInvalidLimit, got %v", err)
		}
	})

	t.Run("valid custom fields", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{"fields": {"src_ip,protocol"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !reflect.DeepEqual(p.Columns, []string{"src_ip", "protocol"}) {
			t.Errorf("got %v", p.Columns)
		}
	})

	t.Run("invalid field rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"fields": {"password"}})
		if !errors.Is(err, ErrInvalidField) {
			t.Errorf("want ErrInvalidField, got %v", err)
		}
	})

	t.Run("valid cursor parsed", func(t *testing.T) {
		ts := "2024-01-15T10:00:00Z"
		p, err := parseListPacketsParams(url.Values{"cursor": {ts}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Cursor == nil {
			t.Fatal("want non-nil cursor")
		}
		if got := p.Cursor.Format(time.RFC3339); got != ts {
			t.Errorf("want %s, got %s", ts, got)
		}
	})

	t.Run("invalid cursor rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"cursor": {"not-a-time"}})
		if !errors.Is(err, ErrCursor) {
			t.Errorf("want ErrCursor, got %v", err)
		}
	})

	t.Run("desc order", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{"order": {"desc"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if p.Order != "DESC" {
			t.Errorf("want DESC, got %s", p.Order)
		}
	})

	t.Run("invalid order rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"order": {"sideways"}})
		if !errors.Is(err, ErrOrder) {
			t.Errorf("want ErrOrder, got %v", err)
		}
	})

	t.Run("valid filter parsed", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{"filter": {"protocol:eq:TCP"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.Filters) != 1 || p.Filters[0].Field != "protocol" || p.Filters[0].Value != "TCP" {
			t.Errorf("unexpected filters: %+v", p.Filters)
		}
	})

	t.Run("invalid filter rejected", func(t *testing.T) {
		_, err := parseListPacketsParams(url.Values{"filter": {"badformat"}})
		if !errors.Is(err, ErrFilter) {
			t.Errorf("want ErrFilter, got %v", err)
		}
	})

	t.Run("multiple filters all parsed", func(t *testing.T) {
		p, err := parseListPacketsParams(url.Values{"filter": {"protocol:eq:TCP", "length:gt:500"}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(p.Filters) != 2 {
			t.Errorf("want 2 filters, got %d", len(p.Filters))
		}
	})
}


func TestBuildPacketQuery(t *testing.T) {
	t.Run("no cursor no filters produces simple query", func(t *testing.T) {
		p := ListPacketsParams{Limit: 10, Columns: []string{"time", "src_ip"}, Order: "ASC"}
		q, args := buildPacketQuery(p)
		want := "SELECT time, src_ip FROM packet_logs ORDER BY time ASC LIMIT $1"
		if q != want {
			t.Errorf("got  %q\nwant %q", q, want)
		}
		if len(args) != 1 || args[0] != 10 {
			t.Errorf("want [10], got %v", args)
		}
	})

	t.Run("cursor adds WHERE clause as first arg", func(t *testing.T) {
		ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		p := ListPacketsParams{Limit: 10, Columns: []string{"time"}, Order: "ASC", Cursor: &ts}
		q, args := buildPacketQuery(p)
		want := "SELECT time FROM packet_logs WHERE time > $1 ORDER BY time ASC LIMIT $2"
		if q != want {
			t.Errorf("got  %q\nwant %q", q, want)
		}
		if len(args) != 2 {
			t.Errorf("want 2 args (cursor + limit), got %d: %v", len(args), args)
		}
	})

	t.Run("filter adds WHERE clause", func(t *testing.T) {
		p := ListPacketsParams{
			Limit:   5,
			Columns: []string{"time"},
			Order:   "DESC",
			Filters: []Filter{{Field: "protocol", Operator: "=", Value: "TCP"}},
		}
		q, args := buildPacketQuery(p)
		want := "SELECT time FROM packet_logs WHERE protocol = $1 ORDER BY time DESC LIMIT $2"
		if q != want {
			t.Errorf("got  %q\nwant %q", q, want)
		}
		if len(args) != 2 || args[0] != "TCP" || args[1] != 5 {
			t.Errorf("unexpected args: %v", args)
		}
	})

	t.Run("cursor and filter combined with AND", func(t *testing.T) {
		ts := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
		p := ListPacketsParams{
			Limit:   20,
			Columns: []string{"time", "protocol"},
			Order:   "ASC",
			Cursor:  &ts,
			Filters: []Filter{{Field: "length", Operator: ">", Value: "500"}},
		}
		q, args := buildPacketQuery(p)
		want := "SELECT time, protocol FROM packet_logs WHERE time > $1 AND length > $2 ORDER BY time ASC LIMIT $3"
		if q != want {
			t.Errorf("got  %q\nwant %q", q, want)
		}
		if len(args) != 3 {
			t.Errorf("want 3 args, got %d: %v", len(args), args)
		}
	})

	t.Run("multiple filters use sequential arg positions", func(t *testing.T) {
		p := ListPacketsParams{
			Limit:   10,
			Columns: []string{"time"},
			Order:   "ASC",
			Filters: []Filter{
				{Field: "protocol", Operator: "=", Value: "TCP"},
				{Field: "length", Operator: ">", Value: "100"},
			},
		}
		q, args := buildPacketQuery(p)
		want := "SELECT time FROM packet_logs WHERE protocol = $1 AND length > $2 ORDER BY time ASC LIMIT $3"
		if q != want {
			t.Errorf("got  %q\nwant %q", q, want)
		}
		if len(args) != 3 {
			t.Errorf("want 3 args, got %d", len(args))
		}
	})
}


func TestPacketQueryDb(t *testing.T) {
	baseParams := ListPacketsParams{
		Limit:   2,
		Columns: []string{"time", "protocol"},
		Order:   "ASC",
	}

	t.Run("DB error returned", func(t *testing.T) {
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return nil, errors.New("connection refused")
			},
		}
		_, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("empty result returns empty data and no cursor", func(t *testing.T) {
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return &mockRows{scanFn: scanTimeProtocol}, nil
			},
		}
		resp, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Data) != 0 {
			t.Errorf("want 0 rows, got %d", len(resp.Data))
		}
		if resp.NextCursor != nil {
			t.Errorf("want nil cursor, got %v", *resp.NextCursor)
		}
	})

	t.Run("rows mapped to packets correctly", func(t *testing.T) {
		ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		rows := &mockRows{
			rows: []storage.PacketRow{
				{Time: ts, Protocol: "TCP"},
				{Time: ts.Add(time.Second), Protocol: "UDP"},
			},
			scanFn: scanTimeProtocol,
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		resp, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Data) != 2 {
			t.Fatalf("want 2 rows, got %d", len(resp.Data))
		}
		if *resp.Data[0].Protocol != "TCP" {
			t.Errorf("want TCP, got %s", *resp.Data[0].Protocol)
		}
		if *resp.Data[1].Protocol != "UDP" {
			t.Errorf("want UDP, got %s", *resp.Data[1].Protocol)
		}
	})

	t.Run("next cursor set when row count equals limit", func(t *testing.T) {
		ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		rows := &mockRows{
			rows: []storage.PacketRow{
				{Time: ts, Protocol: "TCP"},
				{Time: ts.Add(time.Second), Protocol: "UDP"},
			},
			scanFn: scanTimeProtocol,
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		resp, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextCursor == nil {
			t.Fatal("want next_cursor set, got nil")
		}
		want := ts.Add(time.Second).Format(time.RFC3339)
		if *resp.NextCursor != want {
			t.Errorf("want %s, got %s", want, *resp.NextCursor)
		}
	})

	t.Run("next cursor nil when fewer rows than limit", func(t *testing.T) {
		ts := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		rows := &mockRows{
			rows:   []storage.PacketRow{{Time: ts, Protocol: "TCP"}},
			scanFn: scanTimeProtocol,
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		resp, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resp.NextCursor != nil {
			t.Errorf("want nil cursor, got %v", *resp.NextCursor)
		}
	})

	t.Run("scan error returned", func(t *testing.T) {
		rows := &mockRows{
			rows: []storage.PacketRow{{Time: time.Now(), Protocol: "TCP"}},
			scanFn: func(row storage.PacketRow, dest []any) error {
				return errors.New("scan failed")
			},
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		_, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err == nil {
			t.Fatal("want error from scan, got nil")
		}
	})

	t.Run("row iteration error returned", func(t *testing.T) {
		rows := &mockRows{
			iterErr: errors.New("connection dropped"),
			scanFn:  scanTimeProtocol,
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		_, err := packetQueryDb(context.Background(), baseParams, db, "SELECT ...", nil)
		if err == nil {
			t.Fatal("want error from rows.Err(), got nil")
		}
	})

	t.Run("all column types scanned correctly", func(t *testing.T) {
		srcIP := netip.MustParseAddr("1.2.3.4")
		dstIP := netip.MustParseAddr("5.6.7.8")
		ts := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
		jobID := pgtype.UUID{Bytes: [16]byte{1}, Valid: true}

		fullParams := ListPacketsParams{
			Limit:   1,
			Columns: []string{"time", "src_ip", "dst_ip", "src_port", "dst_port", "protocol", "length", "tcp_flags", "job_id"},
			Order:   "ASC",
		}
		rows := &mockRows{
			rows: []storage.PacketRow{{
				Time:     ts,
				SrcIP:    srcIP,
				DstIP:    dstIP,
				SrcPort:  1234,
				DstPort:  80,
				Protocol: "TCP",
				Length:   64,
				TCPFlags: 2,
				JobID:    jobID,
			}},
			scanFn: func(row storage.PacketRow, dest []any) error {
				*dest[0].(*time.Time) = row.Time
				*dest[1].(*netip.Addr) = row.SrcIP
				*dest[2].(*netip.Addr) = row.DstIP
				*dest[3].(*int32) = row.SrcPort
				*dest[4].(*int32) = row.DstPort
				*dest[5].(*string) = row.Protocol
				*dest[6].(*int32) = row.Length
				*dest[7].(*int16) = row.TCPFlags
				*dest[8].(*pgtype.UUID) = row.JobID
				return nil
			},
		}
		db := &mockDB{
			queryFn: func(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
				return rows, nil
			},
		}
		resp, err := packetQueryDb(context.Background(), fullParams, db, "SELECT ...", nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(resp.Data) != 1 {
			t.Fatalf("want 1 row, got %d", len(resp.Data))
		}
		p := resp.Data[0]
		if *p.SrcIP != srcIP.String() {
			t.Errorf("SrcIP: want %s, got %s", srcIP.String(), *p.SrcIP)
		}
		if *p.SrcPort != 1234 {
			t.Errorf("SrcPort: want 1234, got %d", *p.SrcPort)
		}
		if *p.Protocol != "TCP" {
			t.Errorf("Protocol: want TCP, got %s", *p.Protocol)
		}
		if *p.TcpFlags != 2 {
			t.Errorf("TcpFlags: want 2, got %d", *p.TcpFlags)
		}
	})
}
