package api

import (
	"reflect"
	"testing"
)

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
			name:          "stream_id is a valid field",
			input:         []string{"src_ip", "stream_id"},
			wantResult:    []string{"src_ip", "stream_id"},
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
