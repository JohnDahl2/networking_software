package worker

import (
    "io"
	"fmt"
    "os"
    "bufio"
    "sync"

    "github.com/google/gopacket/pcapgo"
)

func PcapWorker(id int, jobs <-chan string, packetStream chan<- []string, wg *sync.WaitGroup) {
    defer wg.Done()
    for path := range jobs {
        batchLimit := 500
        
        err := func(p string) error {
            fmt.Printf("Worker %d: starting file %s\n", id, p)
            f, err := os.Open(p)
            if err != nil {
                return err
            }
            defer f.Close()

            bufferedReader := bufio.NewReader(f)
            reader, err := pcapgo.NewReader(bufferedReader)
            if err != nil {
                return err
            }

            currentBatch := make([]string, 0, batchLimit)

            for {
                data, _, err := reader.ReadPacketData()
                if err == io.EOF {
                    break
                }
                if err != nil {
                    continue
                }

                currentBatch = append(currentBatch, string(data))
                if len(currentBatch) >= batchLimit {
                    packetStream <- currentBatch
                    currentBatch = make([]string, 0, batchLimit)
                }
            }
            if len(currentBatch) > 0 {
                packetStream <- currentBatch
            }

            return nil
        }(path)

        if err != nil {
            fmt.Printf("Worker %d: Error processing %s: %v\n", id, path, err)
        }
    }
}
