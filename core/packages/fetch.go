package packages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"windeploy/internal/atomicfile"
	"windeploy/internal/buildinfo"
)

// ErrHashMismatch reports a downloaded artifact that does not match the pinned
// digest. The partial file is removed before the error is returned.
var ErrHashMismatch = fmt.Errorf("artifact checksum does not match the pinned SHA-256")

// DownloadRequest describes one artifact transfer. ExpectedSHA256 is optional:
// when empty the caller must apply a signature-based trust policy.
type DownloadRequest struct {
	URL            string
	Destination    string
	ExpectedSHA256 string
	MaxBytes       int64
	Progress       func(done, total int64)
}

// DownloadResult is the safe outcome of a completed transfer.
type DownloadResult struct {
	Path   string
	Size   int64
	SHA256 string
}

// Fetcher downloads artifacts. Implementations must write atomically and may not
// leave a failed partial file at Destination.
type Fetcher interface {
	Download(context.Context, DownloadRequest) (DownloadResult, error)
}

// HTTPFetcher downloads over HTTPS only. It streams the body through SHA-256 so a
// large installer never has to be buffered in memory.
type HTTPFetcher struct {
	Client    *http.Client
	UserAgent string
	MaxBytes  int64
}

// NewHTTPFetcher creates a fetcher with conservative defaults.
func NewHTTPFetcher() *HTTPFetcher {
	return &HTTPFetcher{
		Client: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				Proxy:                 http.ProxyFromEnvironment,
				ResponseHeaderTimeout: 45 * time.Second,
				TLSHandshakeTimeout:   15 * time.Second,
				IdleConnTimeout:       60 * time.Second,
				ForceAttemptHTTP2:     true,
			},
		},
		UserAgent: "WinDeploy/" + buildinfo.DisplayVersion(),
		MaxBytes:  2 << 30,
	}
}

// Download implements Fetcher.
func (f *HTTPFetcher) Download(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return DownloadResult{}, fmt.Errorf("installer URL must be a plain https URL")
	}
	if req.Destination == "" {
		return DownloadResult{}, fmt.Errorf("download requires a destination")
	}
	if req.ExpectedSHA256 != "" && !validHexHash(req.ExpectedSHA256) {
		return DownloadResult{}, fmt.Errorf("download requires a valid SHA-256")
	}
	if err := os.MkdirAll(filepath.Dir(req.Destination), 0o755); err != nil {
		return DownloadResult{}, fmt.Errorf("could not create download directory")
	}
	limit := f.MaxBytes
	if req.MaxBytes > 0 && (limit == 0 || req.MaxBytes < limit) {
		limit = req.MaxBytes
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("could not create download request")
	}
	httpReq.Header.Set("User-Agent", f.UserAgent)
	httpReq.Header.Set("Accept", "application/octet-stream")
	client := f.Client
	if client == nil {
		client = http.DefaultClient
	}
	response, err := client.Do(httpReq)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return DownloadResult{}, fmt.Errorf("download failed with HTTP status %d", response.StatusCode)
	}
	if response.ContentLength > limit {
		return DownloadResult{}, fmt.Errorf("artifact exceeds the size limit")
	}
	temp, err := os.CreateTemp(filepath.Dir(req.Destination), ".wd-download-*")
	if err != nil {
		return DownloadResult{}, fmt.Errorf("could not create temporary download file")
	}
	tempName := temp.Name()
	cleanup := func() {
		temp.Close()
		os.Remove(tempName)
	}
	hasher := sha256.New()
	writer := io.MultiWriter(temp, hasher)
	var done int64
	buffer := make([]byte, 128*1024)
	for {
		if err := ctx.Err(); err != nil {
			cleanup()
			return DownloadResult{}, err
		}
		read, readErr := response.Body.Read(buffer)
		if read > 0 {
			done += int64(read)
			if done > limit {
				cleanup()
				return DownloadResult{}, fmt.Errorf("artifact exceeds the size limit")
			}
			if _, err := writer.Write(buffer[:read]); err != nil {
				cleanup()
				return DownloadResult{}, fmt.Errorf("could not write downloaded artifact")
			}
			if req.Progress != nil {
				req.Progress(done, response.ContentLength)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			cleanup()
			return DownloadResult{}, fmt.Errorf("download was interrupted")
		}
	}
	if err := temp.Sync(); err != nil {
		cleanup()
		return DownloadResult{}, fmt.Errorf("could not flush downloaded artifact")
	}
	digest := hex.EncodeToString(hasher.Sum(nil))
	if req.ExpectedSHA256 != "" && !strings.EqualFold(digest, req.ExpectedSHA256) {
		cleanup()
		return DownloadResult{}, ErrHashMismatch
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		return DownloadResult{}, fmt.Errorf("could not close downloaded artifact")
	}
	if err := atomicfile.Replace(ctx, tempName, req.Destination); err != nil {
		os.Remove(tempName)
		return DownloadResult{}, fmt.Errorf("could not store downloaded artifact")
	}
	return DownloadResult{Path: req.Destination, Size: done, SHA256: digest}, nil
}
