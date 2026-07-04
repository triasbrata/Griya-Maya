package convert

import (
	"context"
	"runtime"
	"sync"

	"github.com/triasbrata/mihon-manga-server/internal/domain"
)

// Result is one converted page: its AVIF bytes and dimensions.
type Result struct {
	Index  int
	Data   []byte
	Width  int
	Height int
}

// Converter extracts pages from an archive and encodes them to AVIF.
type Converter struct {
	opt EncodeOptions
}

// NewConverter builds a Converter with fixed encode options.
func NewConverter(opt EncodeOptions) *Converter {
	return &Converter{opt: opt}
}

// Convert detects the format, extracts ordered pages and encodes each to AVIF.
// Encoding runs across GOMAXPROCS workers; page order is preserved in output.
func (c *Converter) Convert(ctx context.Context, format domain.ArchiveFormat, archive []byte) ([]Result, error) {
	ext, err := extractorFor(format)
	if err != nil {
		return nil, err
	}
	raw, err := ext.Extract(archive)
	if err != nil {
		return nil, err
	}

	results := make([]Result, len(raw))
	workers := runtime.GOMAXPROCS(0)
	if workers > len(raw) {
		workers = len(raw)
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		firstEr error
	)
	jobs := make(chan int)

	worker := func() {
		defer wg.Done()
		for i := range jobs {
			select {
			case <-ctx.Done():
				mu.Lock()
				if firstEr == nil {
					firstEr = ctx.Err()
				}
				mu.Unlock()
				return
			default:
			}
			enc, err := decodeAndEncode(raw[i].data, i, c.opt)
			if err != nil {
				mu.Lock()
				if firstEr == nil {
					firstEr = err
				}
				mu.Unlock()
				continue
			}
			results[i] = Result{Index: i, Data: enc.Data, Width: enc.Width, Height: enc.Height}
		}
	}

	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go worker()
	}
	for i := range raw {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	if firstEr != nil {
		return nil, firstEr
	}
	return results, nil
}
