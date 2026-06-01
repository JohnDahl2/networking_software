package worker

import (
	"testing"

	"github.com/google/gopacket/layers"
)

func TestPacketFlagsToUint8(t *testing.T) {
	tests := []struct {
		name     string
		tcp      layers.TCP
		expected uint8
	}{
		{
			name:     "no flags set returns zero",
			tcp:      layers.TCP{},
			expected: 0,
		},
		{
			name:     "SYN flag",
			tcp:      layers.TCP{SYN: true},
			expected: 1 << 1,
		},
		{
			name:     "ACK flag",
			tcp:      layers.TCP{ACK: true},
			expected: 1 << 4,
		},
		{
			name:     "FIN flag",
			tcp:      layers.TCP{FIN: true},
			expected: 1 << 0,
		},
		{
			name:     "RST flag",
			tcp:      layers.TCP{RST: true},
			expected: 1 << 2,
		},
		{
			name:     "PSH flag",
			tcp:      layers.TCP{PSH: true},
			expected: 1 << 3,
		},
		{
			name:     "URG flag",
			tcp:      layers.TCP{URG: true},
			expected: 1 << 5,
		},
		{
			name:     "SYN+ACK (typical handshake response)",
			tcp:      layers.TCP{SYN: true, ACK: true},
			expected: (1 << 1) | (1 << 4),
		},
		{
			name:     "FIN+ACK (connection teardown)",
			tcp:      layers.TCP{FIN: true, ACK: true},
			expected: (1 << 0) | (1 << 4),
		},
		{
			name:     "RST+ACK (reset with ack)",
			tcp:      layers.TCP{RST: true, ACK: true},
			expected: (1 << 2) | (1 << 4),
		},
		{
			name:     "all flags set",
			tcp:      layers.TCP{SYN: true, ACK: true, FIN: true, RST: true, PSH: true, URG: true},
			expected: (1 << 1) | (1 << 4) | (1 << 0) | (1 << 2) | (1 << 3) | (1 << 5),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := packetFlagsToUint8(&tc.tcp)
			if result != tc.expected {
				t.Errorf("got %08b, want %08b", result, tc.expected)
			}
		})
	}
}
