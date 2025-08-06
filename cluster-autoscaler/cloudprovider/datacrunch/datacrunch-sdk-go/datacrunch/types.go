package datacrunch

import (
	"io"
)

// ReaderSeekerCloser wraps a io.Reader returning a ReaderSeekerCloser.
// This allows the SDK to accept an io.Reader that is not also an io.Seeker
// for streaming operations.
type ReaderSeekerCloser struct {
	r io.Reader
}

// ReadSeekCloser wraps a io.Reader returning a ReaderSeekerCloser.
func ReadSeekCloser(r io.Reader) ReaderSeekerCloser {
	return ReaderSeekerCloser{r}
}

// Read reads from the wrapped io.Reader.
func (r ReaderSeekerCloser) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

// Seek is a stub that returns an error since the underlying reader is not seekable.
func (r ReaderSeekerCloser) Seek(offset int64, whence int) (int64, error) {
	return 0, io.ErrNoProgress
}

// Close is a stub that does nothing since we don't own the underlying reader.
func (r ReaderSeekerCloser) Close() error {
	if closer, ok := r.r.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// IsReaderSeekable returns true if the reader is seekable.
func IsReaderSeekable(r io.Reader) bool {
	_, ok := r.(io.Seeker)
	return ok
}

// SeekerLen attempts to get the length of the seeker.
func SeekerLen(s io.Seeker) (int64, error) {
	// Get current position
	cur, err := s.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	// Get end position
	end, err := s.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	// Restore original position
	_, err = s.Seek(cur, io.SeekStart)
	if err != nil {
		return 0, err
	}

	return end - cur, nil
}
