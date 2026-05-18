package worker

import (
    "io"
	"fmt"
    "os"
    "bufio"
    "sync"
    "context"

    "github.com/google/gopacket/pcapgo"
)

func PcapWorker(ctx context.Context, id int, jobs <-chan string, packetStream chan<- []string, wg *sync.WaitGroup) {
    defer wg.Done()

    for path := range jobs {
        // 1. ALWAYS check if the system-wide context was cancelled BEFORE starting a new file
        select {
        case <-ctx.Done():
            fmt.Printf("Worker %d: Stopping early, pipeline context cancelled.\n", id)
            return
        default:
            // Keep going normally if context is active
        }

        batchLimit := 500
        
        err := func(p string) error {
            fmt.Printf("Worker %d: starting file %s\n", id, p)
            f, err := os.Open(p)
            if err != nil {
                return err
            }
            defer f.Close() // Safely closes right when this anonymous function ends

            bufferedReader := bufio.NewReader(f)
            reader, err := pcapgo.NewReader(bufferedReader)
            if err != nil {
                return err
            }

            currentBatch := make([]string, 0, batchLimit)

            for {
                // 2. For massive files, check context inside the packet loop too
                // This ensures a 5GB file can stop instantly if you hit Ctrl+C
                if len(currentBatch) == 0 { // Check periodically, not on every single packet
                    select {
                    case <-ctx.Done():
                        return ctx.Err()
                    default:
                    }
                }

                data, _, err := reader.ReadPacketData()
                if err == io.EOF {
                    break
                }
                if err != nil {
                    continue // Skip corrupted packets within a good file
                }

                currentBatch = append(currentBatch, string(data))
                if len(currentBatch) >= batchLimit {
                    // 3. Make sure channel sends respect context cancellation
                    select {
                    case packetStream <- currentBatch:
                    case <-ctx.Done():
                        return ctx.Err()
                    }
                    currentBatch = make([]string, 0, batchLimit)
                }
            }
            if len(currentBatch) > 0 {
                select {
                case packetStream <- currentBatch:
                case <-ctx.Done():
                    return ctx.Err()
                }
            }

            return nil
        }(path)

        // 4. Clean, isolated error tracking
        if err != nil {
            // Since we don't have a logger yet, standard printing works perfectly.
            // Because we don't panic or return here, the loop moves directly 
            // to the NEXT path in the jobs channel. The worker survives!
            fmt.Printf("ERR: Worker %d failed to process file %s: %v\n", id, path, err)
        }
    }
}
