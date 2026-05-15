
package pcap

import (
    "io"
    "log"
	"os"
	"bufio"

	"github.com/google/gopacket/pcapgo"
)


func PcapReader(path string) int {
    f, err := os.Open(path)
    if err != nil {
        log.Printf("Error opening %s: %v", path, err)
        return 0
    }
    defer f.Close()

    bufferedReader := bufio.NewReader(f)
    reader, err := pcapgo.NewReader(bufferedReader)
    if err != nil {
        log.Printf("Error creating pcap reader for %s: %v", path, err)
        return 0
    }

    count := 0
    for {
        _, _, err := reader.ReadPacketData()
        if err == io.EOF {
            break
        }
        if err != nil {
            continue
        }
        count++
    }
    return count
}

