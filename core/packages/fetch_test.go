package packages

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testFetcherFor(server *httptest.Server) *HTTPFetcher {
	fetcher := NewHTTPFetcher()
	fetcher.Client = server.Client()
	return fetcher
}

func TestHTTPFetcherVerifiedDownload(t *testing.T) {
	payload := []byte("verified payload")
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Errorf("missing user agent")
		}
		w.Header().Set("Content-Length", "16")
		_, _ = w.Write(payload)
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	fetcher := testFetcherFor(server)
	result, err := fetcher.Download(context.Background(), DownloadRequest{URL: server.URL + "/artifact.bin", Destination: destination, ExpectedSHA256: HashBytes(payload)})
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if result.Size != int64(len(payload)) || result.SHA256 != HashBytes(payload) {
		t.Fatalf("unexpected result: %+v", result)
	}
	content, err := os.ReadFile(destination)
	if err != nil || string(content) != string(payload) {
		t.Fatalf("stored file mismatch: %v", err)
	}
}

func TestHTTPFetcherRejectsWrongDigest(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	_, err := testFetcherFor(server).Download(context.Background(), DownloadRequest{URL: server.URL + "/artifact.bin", Destination: destination, ExpectedSHA256: strings.Repeat("a", 64)})
	if !errors.Is(err, ErrHashMismatch) {
		t.Fatalf("expected hash mismatch, got %v", err)
	}
	if _, statErr := os.Stat(destination); !os.IsNotExist(statErr) {
		t.Fatal("failed download must not leave the destination file")
	}
}

func TestHTTPFetcherRejectsHTTPAndBadStatus(t *testing.T) {
	fetcher := NewHTTPFetcher()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	if _, err := fetcher.Download(context.Background(), DownloadRequest{URL: "http://example.test/a.exe", Destination: destination, ExpectedSHA256: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("plain HTTP must be rejected")
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer server.Close()
	if _, err := testFetcherFor(server).Download(context.Background(), DownloadRequest{URL: server.URL + "/a.exe", Destination: destination, ExpectedSHA256: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("non-200 response must fail")
	}
}

func TestHTTPFetcherEnforcesSizeLimit(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("0123456789"))
	}))
	defer server.Close()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	fetcher := testFetcherFor(server)
	fetcher.MaxBytes = 4
	if _, err := fetcher.Download(context.Background(), DownloadRequest{URL: server.URL + "/a.exe", Destination: destination, ExpectedSHA256: strings.Repeat("a", 64)}); err == nil {
		t.Fatal("oversized artifact must fail")
	}
}

func TestHTTPFetcherHonorsCancellation(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000")
		flusher, _ := w.(http.Flusher)
		if flusher != nil {
			flusher.Flush()
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	destination := filepath.Join(t.TempDir(), "artifact.bin")
	_, err := testFetcherFor(server).Download(ctx, DownloadRequest{URL: server.URL + "/a.exe", Destination: destination, ExpectedSHA256: strings.Repeat("a", 64)})
	if err == nil {
		t.Fatal("cancelled download must fail")
	}
}
