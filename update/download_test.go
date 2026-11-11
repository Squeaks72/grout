package update

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	uberatomic "go.uber.org/atomic"
)

func testOptions() downloadOptions {
	return downloadOptions{Idle: 200 * time.Millisecond, Attempts: 5, Backoff: time.Millisecond, Budget: 10 * time.Second}
}

func payload() []byte { return bytes.Repeat([]byte("grout-release-bytes-"), 5000) } // 100,000 bytes

// A server that cuts the first response short, then serves the rest on request.
type flakyServer struct {
	mu       sync.Mutex
	data     []byte
	cutAfter int
	ranges   []string
	requests int
}

func (f *flakyServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.requests++
	first := f.requests == 1
	f.ranges = append(f.ranges, r.Header.Get("Range"))
	f.mu.Unlock()

	if first {
		hj := w.(http.Hijacker)
		conn, buf, _ := hj.Hijack()
		buf.WriteString("HTTP/1.1 200 OK\r\nContent-Length: 100000\r\n\r\n")
		buf.Write(f.data[:f.cutAfter])
		buf.Flush()
		conn.Close()
		return
	}
	http.ServeContent(w, r, "x.zip", time.Time{}, bytes.NewReader(f.data))
}

func TestDownloadResumable_CarriesOnAfterTheConnectionDrops(t *testing.T) {
	srv := &flakyServer{data: payload(), cutAfter: 30000}
	ts := httptest.NewServer(srv)
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")

	err := downloadResumable(ts.URL, path, int64(len(srv.data)), &uberatomic.Float64{}, testOptions())
	if err != nil {
		t.Fatalf("downloadResumable: %v", err)
	}

	got, _ := os.ReadFile(path)
	if !bytes.Equal(got, srv.data) {
		t.Fatalf("file is %d bytes and does not match the source (%d)", len(got), len(srv.data))
	}
	if len(srv.ranges) < 2 || srv.ranges[1] != "bytes=30000-" {
		t.Errorf("second request asked for %q, want to resume at byte 30000", srv.ranges)
	}
}

func TestDownloadResumable_ResumesFromAPartFileLeftByAnEarlierRun(t *testing.T) {
	data := payload()
	var ranges []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ranges = append(ranges, r.Header.Get("Range"))
		http.ServeContent(w, r, "x.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")
	os.WriteFile(path, data[:60000], 0644)

	if err := downloadResumable(ts.URL, path, int64(len(data)), nil, testOptions()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatal("resumed file does not match")
	}
	if len(ranges) != 1 || ranges[0] != "bytes=60000-" {
		t.Errorf("requests = %q, want one from byte 60000", ranges)
	}
}

// A server that ignores the Range header sends everything again; the file must
// not end up with the first part twice.
func TestDownloadResumable_ServerThatIgnoresRangeRestarts(t *testing.T) {
	data := payload()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")
	os.WriteFile(path, data[:60000], 0644)

	if err := downloadResumable(ts.URL, path, int64(len(data)), nil, testOptions()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatalf("file is %d bytes, want %d", len(got), len(data))
	}
}

func TestDownloadResumable_DiscardsAPartFileLargerThanTheDownload(t *testing.T) {
	data := payload()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "x.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")
	os.WriteFile(path, bytes.Repeat([]byte("z"), len(data)+500), 0644)

	if err := downloadResumable(ts.URL, path, int64(len(data)), nil, testOptions()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatal("stale partial was not replaced")
	}
}

// A connection that stops delivering without closing is given up on after the
// idle time, and the next attempt carries on.
func TestDownloadResumable_GivesUpOnAStalledAttemptAndResumes(t *testing.T) {
	data := payload()
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&requests, 1) == 1 {
			w.Header().Set("Content-Length", "100000")
			w.Write(data[:20000])
			w.(http.Flusher).Flush()
			<-r.Context().Done() // the client gives up
			return
		}
		http.ServeContent(w, r, "x.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")

	if err := downloadResumable(ts.URL, path, int64(len(data)), nil, testOptions()); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, data) {
		t.Fatal("file does not match after a stall")
	}
}

func TestDownloadResumable_DoesNotRetryAServerThatRefuses(t *testing.T) {
	var requests int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requests, 1)
		http.NotFound(w, r)
	}))
	defer ts.Close()

	err := downloadResumable(ts.URL, filepath.Join(t.TempDir(), "x.part"), 1000, nil, testOptions())

	if err == nil {
		t.Fatal("a 404 was treated as success")
	}
	if requests != 1 {
		t.Errorf("requests = %d, want 1: a missing file is not worth retrying", requests)
	}
}

func TestDownloadResumable_ReportsProgressFromWhatIsAlreadyThere(t *testing.T) {
	data := payload()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "x.zip", time.Time{}, bytes.NewReader(data))
	}))
	defer ts.Close()
	path := filepath.Join(t.TempDir(), "x.part")
	os.WriteFile(path, data[:50000], 0644)
	progress := &uberatomic.Float64{}

	if err := downloadResumable(ts.URL, path, int64(len(data)), progress, testOptions()); err != nil {
		t.Fatal(err)
	}
	if got := progress.Load(); got < 0.999 {
		t.Errorf("final progress = %v, want 1", got)
	}
}

func TestRemoveOtherPartials_KeepsOnlyThisReleases(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	keep := partialDownloadPath(strings.Repeat("a", 64))
	other := partialDownloadPath(strings.Repeat("b", 64))
	os.WriteFile(keep, []byte("x"), 0644)
	os.WriteFile(other, []byte("y"), 0644)

	removeOtherPartials(keep)

	if _, err := os.Stat(keep); err != nil {
		t.Error("this release's partial was removed")
	}
	if _, err := os.Stat(other); !os.IsNotExist(err) {
		t.Error("another release's partial was kept")
	}
}
