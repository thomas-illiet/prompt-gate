package proxylimits

import (
	"errors"
	"io"
	"time"
)

const (
	DefaultMaxBufferedRequestBytes  int64 = 32 * 1024 * 1024
	DefaultMaxBufferedResponseBytes int64 = 64 * 1024 * 1024
	DefaultUpstreamTimeout                = 2 * time.Minute
)

var ErrExceeded = errors.New("proxy buffer limit exceeded")

// ReadAll reads reader up to maxBytes and returns ErrExceeded when the limit is crossed.
func ReadAll(reader io.Reader, maxBytes int64) ([]byte, error) {
	if maxBytes <= 0 {
		return io.ReadAll(reader)
	}

	raw, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxBytes {
		return nil, ErrExceeded
	}
	return raw, nil
}
