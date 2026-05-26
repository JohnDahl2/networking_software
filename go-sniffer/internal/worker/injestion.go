package worker

import (
	"bufio"
	"context"
	"io"
	"log/slog"
	"strings"
    "sync/atomic"
	"os"
	"sync"

	"github.com/google/gopacket/pcapgo"
)

var TotalPacketsRead int64

func PcapWorker(ctx context.Context, id int, jobs <-chan string, packetStream chan<- []string, wg *sync.WaitGroup) {
	defer wg.Done()

	// Create a localized contextual logger for this reader thread.
	// Every log line called with 'log.' will automatically include the "reader_id" key.
	log := slog.With("reader_id", id)

	for path := range jobs {
		// 1. ALWAYS check if the system-wide context was cancelled BEFORE starting a new file
		select {
		case <-ctx.Done():
			log.Warn("pipeline context cancelled; stopping reader thread early")
			return
		default:
			// Keep going normally if context is active
		}

		batchLimit := 500
		
		err := func(p string) error {
			// Set to DEBUG level so file rotation logs stay out of standard production noise
			log.Debug("starting file extraction sequence", "file_path", p)
			
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
				// 2. For massive files, check context inside the packet loop periodically
				if len(currentBatch) == 0 { 
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
					// If the error message contains "EOF", it means we safely hit the end of the file 
					// structure but the reader loop just wants to exit. Break out cleanly!
					if strings.Contains(err.Error(), "EOF") {
						break
					}

					// Real structural corruptions will still surface here cleanly
					log.Debug("skipping corrupted packet within valid file structure", "file_path", p, "error", err.Error())
					continue 
				}
				currentBatch = append(currentBatch, string(data))
				if len(currentBatch) >= batchLimit {
					// 3. Make sure channel sends respect context cancellation
					select {
					case packetStream <- currentBatch:
					case <-ctx.Done():
						return ctx.Err()
					}
                    atomic.AddInt64(&TotalPacketsRead, int64(len(currentBatch)))
                    currentBatch = make([]string, 0, batchLimit)
				}
			}
			
			if len(currentBatch) > 0 {
				select {
				case packetStream <- currentBatch:
				case <-ctx.Done():
					return ctx.Err()
				}
                atomic.AddInt64(&TotalPacketsRead, int64(len(currentBatch)))
			}

			return nil
		}(path)

		// 4. Clean, isolated error tracking
		if err != nil {
			// If the error was just a deliberate context cancellation, log it as a quiet note, not a massive error failure
			if ctx.Err() != nil {
				log.Debug("file extraction aborted cleanly by application context shutdown", "file_path", path)
			} else {
				log.Error("failed to process target pcap file completely", "file_path", path, "error", err.Error())
			}
		}
	}
}