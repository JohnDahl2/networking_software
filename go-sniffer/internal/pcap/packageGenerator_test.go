package pcap

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/pcapgo"
)

// readPcapPackets opens a pcap file and returns all packet data and capture infos.
func readPcapPackets(t *testing.T, path string) ([][]byte, []gopacket.CaptureInfo) {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open pcap file: %v", err)
	}
	defer f.Close() //nolint:errcheck

	reader, err := pcapgo.NewReader(f)
	if err != nil {
		t.Fatalf("create pcap reader: %v", err)
	}

	var packets [][]byte
	var infos []gopacket.CaptureInfo
	for {
		data, ci, err := reader.ReadPacketData()
		if err != nil {
			break
		}
		packets = append(packets, data)
		infos = append(infos, ci)
	}
	return packets, infos
}

// --- ensureDir ---

func TestEnsureDir(t *testing.T) {
	t.Run("creates nested directory when it does not exist", func(t *testing.T) {
		dir := t.TempDir()
		target := filepath.Join(dir, "a", "b", "c")

		if err := ensureDir(target); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(target); err != nil {
			t.Fatalf("directory was not created: %v", err)
		}
	})

	t.Run("returns nil when directory already exists", func(t *testing.T) {
		dir := t.TempDir()
		if err := ensureDir(dir); err != nil {
			t.Fatalf("unexpected error for existing directory: %v", err)
		}
	})

	t.Run("returns error when path cannot be created", func(t *testing.T) {
		dir := t.TempDir()
		// Make a read-only directory so MkdirAll cannot create a child inside it.
		readOnly := filepath.Join(dir, "readonly")
		if err := os.Mkdir(readOnly, 0555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(readOnly, 0755) }) //nolint:errcheck // restore so TempDir cleanup can delete it
		if err := ensureDir(filepath.Join(readOnly, "subdir")); err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}

// --- Generate ---

var baseTime = time.Date(2026, time.March, 7, 14, 30, 0, 0, time.UTC)

var udpProfile = TrafficProfile{
	Name:       "UDP Test",
	Proto:      "UDP",
	SrcIP:      []byte{10, 0, 0, 1},
	DstIP:      []byte{10, 0, 0, 2},
	SrcPort:    1234,
	DstPort:    5678,
	PayloadMin: 100,
	PayloadMax: 200,
}

var tcpRSTProfile = TrafficProfile{
	Name:     "TCP RST Test",
	Proto:    "TCP",
	SrcIP:    []byte{10, 0, 0, 1},
	DstIP:    []byte{10, 0, 0, 2},
	SrcPort:  12345,
	DstPort:  443,
	TCPFlags: TCPFlags{RST: true},
}

var tcpPayloadProfile = TrafficProfile{
	Name:       "TCP Payload Test",
	Proto:      "TCP",
	SrcIP:      []byte{10, 0, 0, 1},
	DstIP:      []byte{10, 0, 0, 2},
	SrcPort:    12345,
	DstPort:    80,
	PayloadMin: 50,
	PayloadMax: 100,
}

func TestGenerate(t *testing.T) {
	t.Run("creates a valid pcap file with the correct packet count", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		if err := Generate(out, 10, baseTime, []TrafficProfile{udpProfile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		packets, _ := readPcapPackets(t, out)
		if len(packets) != 10 {
			t.Errorf("expected 10 packets, got %d", len(packets))
		}
	})

	t.Run("timestamps increment by 10ms per packet", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		profile := TrafficProfile{
			Proto:      "UDP",
			SrcIP:      []byte{10, 0, 0, 1},
			DstIP:      []byte{10, 0, 0, 2},
			SrcPort:    1234,
			DstPort:    5678,
			PayloadMin: 10,
			PayloadMax: 10,
		}

		if err := Generate(out, 5, baseTime, []TrafficProfile{profile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		_, infos := readPcapPackets(t, out)
		if len(infos) != 5 {
			t.Fatalf("expected 5 packets, got %d", len(infos))
		}
		for i := 1; i < len(infos); i++ {
			diff := infos[i].Timestamp.Sub(infos[i-1].Timestamp)
			if diff != 10*time.Millisecond {
				t.Errorf("packet %d: expected 10ms gap, got %v", i, diff)
			}
		}
	})

	t.Run("writes zero packets when packetCount is zero", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		if err := Generate(out, 0, baseTime, []TrafficProfile{udpProfile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		packets, _ := readPcapPackets(t, out)
		if len(packets) != 0 {
			t.Errorf("expected 0 packets, got %d", len(packets))
		}
	})

	t.Run("generates valid packets for TCP RST profile", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		if err := Generate(out, 5, baseTime, []TrafficProfile{tcpRSTProfile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		packets, _ := readPcapPackets(t, out)
		if len(packets) != 5 {
			t.Errorf("expected 5 packets, got %d", len(packets))
		}
	})

	t.Run("generates valid packets for TCP profile with payload", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		if err := Generate(out, 5, baseTime, []TrafficProfile{tcpPayloadProfile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		packets, _ := readPcapPackets(t, out)
		if len(packets) != 5 {
			t.Errorf("expected 5 packets, got %d", len(packets))
		}
	})

	t.Run("mixed TCP and UDP profiles all produce packets", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		profiles := []TrafficProfile{udpProfile, tcpRSTProfile, tcpPayloadProfile}
		if err := Generate(out, 30, baseTime, profiles); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		packets, _ := readPcapPackets(t, out)
		if len(packets) != 30 {
			t.Errorf("expected 30 packets, got %d", len(packets))
		}
	})

	t.Run("returns error for invalid file path", func(t *testing.T) {
		err := Generate("/nonexistent/path/test.pcap", 1, baseTime, []TrafficProfile{udpProfile})
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
	})

	t.Run("output file is non-empty", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "test.pcap")

		if err := Generate(out, 5, baseTime, []TrafficProfile{udpProfile}); err != nil {
			t.Fatalf("Generate returned error: %v", err)
		}

		info, err := os.Stat(out)
		if err != nil {
			t.Fatalf("stat output file: %v", err)
		}
		if info.Size() == 0 {
			t.Error("expected non-empty output file")
		}
	})
}

// --- GenerateFiles ---

func TestGenerateFiles(t *testing.T) {
	t.Run("creates the correct number of files", func(t *testing.T) {
		dir := t.TempDir()
		GenerateFiles(3, 1, dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "*.pcap"))
		if len(matches) != 3 {
			t.Errorf("expected 3 files, got %d", len(matches))
		}
	})

	t.Run("files are named test_batch_N.pcap", func(t *testing.T) {
		dir := t.TempDir()
		GenerateFiles(2, 1, dir)

		for i := 1; i <= 2; i++ {
			expected := filepath.Join(dir, fmt.Sprintf("test_batch_%d.pcap", i))
			if _, err := os.Stat(expected); err != nil {
				t.Errorf("expected file %s not found", expected)
			}
		}
	})

	t.Run("skips generation when enough files already exist", func(t *testing.T) {
		dir := t.TempDir()
		for i := 1; i <= 3; i++ {
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("test_batch_%d.pcap", i)))
			if err != nil {
				t.Fatal(err)
			}
			f.Close() //nolint:errcheck
		}

		GenerateFiles(3, 1, dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "*.pcap"))
		if len(matches) != 3 {
			t.Errorf("expected still 3 files, got %d", len(matches))
		}
	})

	t.Run("generates only missing files when partially filled", func(t *testing.T) {
		dir := t.TempDir()
		// Pre-create 1 file, ask for 3 total — should generate 2 more.
		f, err := os.Create(filepath.Join(dir, "test_batch_1.pcap"))
		if err != nil {
			t.Fatal(err)
		}
		f.Close() //nolint:errcheck

		GenerateFiles(3, 1, dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "*.pcap"))
		if len(matches) != 3 {
			t.Errorf("expected 3 files, got %d", len(matches))
		}
	})

	t.Run("uses default count of 3 when count is zero", func(t *testing.T) {
		dir := t.TempDir()
		GenerateFiles(0, 1, dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "*.pcap"))
		if len(matches) != 3 {
			t.Errorf("expected 3 files (default count), got %d", len(matches))
		}
	})

	t.Run("each generated file is a valid pcap", func(t *testing.T) {
		dir := t.TempDir()
		GenerateFiles(2, 1, dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "*.pcap"))
		for _, path := range matches {
			f, err := os.Open(path)
			if err != nil {
				t.Errorf("open %s: %v", path, err)
				continue
			}
			_, err = pcapgo.NewReader(f)
			f.Close() //nolint:errcheck
			if err != nil {
				t.Errorf("file %s is not a valid pcap: %v", path, err)
			}
		}
	})
}

// --- RemoveFiles ---

func TestRemoveFiles(t *testing.T) {
	t.Run("removes all test_batch_*.pcap files", func(t *testing.T) {
		dir := t.TempDir()
		for i := 1; i <= 3; i++ {
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("test_batch_%d.pcap", i)))
			if err != nil {
				t.Fatal(err)
			}
			f.Close() //nolint:errcheck
		}

		RemoveFiles(dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "test_batch_*.pcap"))
		if len(matches) != 0 {
			t.Errorf("expected 0 files after removal, got %d", len(matches))
		}
	})

	t.Run("does nothing when no matching files exist", func(t *testing.T) {
		dir := t.TempDir()
		// Should not panic or error — verified by test completing cleanly.
		RemoveFiles(dir)
	})

	t.Run("does not remove non-matching pcap files", func(t *testing.T) {
		dir := t.TempDir()
		keep := filepath.Join(dir, "important.pcap")
		if err := os.WriteFile(keep, []byte("data"), 0644); err != nil {
			t.Fatal(err)
		}

		RemoveFiles(dir)

		if _, err := os.Stat(keep); err != nil {
			t.Error("non-matching file was incorrectly removed")
		}
	})

	t.Run("removes files and count matches", func(t *testing.T) {
		dir := t.TempDir()
		for i := 1; i <= 5; i++ {
			f, err := os.Create(filepath.Join(dir, fmt.Sprintf("test_batch_%d.pcap", i)))
			if err != nil {
				t.Fatal(err)
			}
			f.Close() //nolint:errcheck
		}

		RemoveFiles(dir)

		matches, _ := filepath.Glob(filepath.Join(dir, "test_batch_*.pcap"))
		if len(matches) != 0 {
			t.Errorf("expected 0 files, got %d", len(matches))
		}
	})
}
