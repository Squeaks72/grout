package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"go.uber.org/atomic"
)

// A download on a slow or unsteady link is expected to stall or drop part-way.
// It carries on from what has arrived instead of starting again, and gives up
// on an attempt that has stopped getting data rather than on one that is
// merely slow.
type downloadOptions struct {
	// Idle is how long an attempt may go without receiving a byte.
	Idle time.Duration
	// Attempts is how many times to try in all.
	Attempts int
	// Backoff is the wait before each further attempt.
	Backoff time.Duration
	// Budget is the longest the whole download may take, attempts included.
	Budget time.Duration
}

func defaultDownloadOptions() downloadOptions {
	return downloadOptions{
		Idle:     30 * time.Second,
		Attempts: 10,
		Backoff:  3 * time.Second,
		Budget:   45 * time.Minute,
	}
}

const partialPrefix = "grout-update-"

// partialDownloadPath is where a release's download is kept while it arrives.
func partialDownloadPath(sha256 string) string {
	id := sha256
	if len(id) > 16 {
		id = id[:16]
	}
	return filepath.Join(os.TempDir(), partialPrefix+id+".part")
}

// removeOtherPartials deletes downloads of other releases, which would only
// take up room.
func removeOtherPartials(keep string) {
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), partialPrefix+"*.part"))
	for _, m := range matches {
		if m != keep {
			os.Remove(m)
		}
	}
}

// downloadResumable fetches url into path, resuming from whatever is already
// there. A server that ignores the request to resume is handled by starting
// again. The result is not checked here: the caller verifies the checksum.
func downloadResumable(url, path string, expectedSize int64, progress *atomic.Float64, opts downloadOptions) error {
	ctx, cancel := context.WithTimeout(context.Background(), opts.Budget)
	defer cancel()

	var lastErr error
	for attempt := 1; attempt <= opts.Attempts; attempt++ {
		done, err := downloadAttempt(ctx, url, path, expectedSize, progress, opts.Idle)
		if err == nil && done {
			return nil
		}
		if err == nil {
			err = errors.New("download ended before the whole file arrived")
		}
		lastErr = err
		if ctx.Err() != nil || errors.Is(err, errNotResumable) {
			break
		}
		if attempt < opts.Attempts {
			select {
			case <-time.After(opts.Backoff):
			case <-ctx.Done():
			}
		}
	}
	return lastErr
}

var errNotResumable = errors.New("the server refused the download")

// downloadAttempt makes one request, from the end of what is already on disk.
// It reports whether the file is now complete.
func downloadAttempt(ctx context.Context, url, path string, expectedSize int64, progress *atomic.Float64, idle time.Duration) (bool, error) {
	var have int64
	if info, err := os.Stat(path); err == nil {
		have = info.Size()
	}
	if expectedSize > 0 && have > expectedSize {
		// Not part of this download.
		os.Remove(path)
		have = 0
	}
	if expectedSize > 0 && have == expectedSize {
		return true, nil
	}

	// The idle timer cancels the request when no data has arrived for a while.
	reqCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	timer := time.AfterFunc(idle, cancel)
	defer timer.Stop()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, url, nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("User-Agent", "Grout-Updater")
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	flags := os.O_WRONLY | os.O_CREATE
	switch {
	case resp.StatusCode == http.StatusPartialContent && have > 0:
		flags |= os.O_APPEND
	case resp.StatusCode == http.StatusOK:
		// Either a first attempt or a server that sent the whole file again.
		flags |= os.O_TRUNC
		have = 0
	case resp.StatusCode == http.StatusRequestedRangeNotSatisfiable:
		// What is on disk is not a prefix of this file. Start over next time.
		os.Remove(path)
		return false, errors.New("the partial download did not match, starting again")
	case resp.StatusCode >= 400 && resp.StatusCode < 500:
		return false, fmt.Errorf("%w: status %d", errNotResumable, resp.StatusCode)
	default:
		return false, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	out, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		return false, err
	}
	defer out.Close()

	total := expectedSize
	if total <= 0 {
		total = have + resp.ContentLength
	}
	if total <= 0 {
		total = 1
	}

	written := have
	setProgress := func() {
		if progress != nil {
			progress.Store(float64(written) / float64(total))
		}
	}
	setProgress()

	buf := make([]byte, 32*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			timer.Reset(idle)
			if _, werr := out.Write(buf[:n]); werr != nil {
				return false, werr
			}
			written += int64(n)
			setProgress()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return false, readErr
		}
	}

	if expectedSize > 0 && written != expectedSize {
		return false, nil
	}
	return true, nil
}
