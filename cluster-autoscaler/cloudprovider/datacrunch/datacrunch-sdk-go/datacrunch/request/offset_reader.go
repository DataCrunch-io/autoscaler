package request

import (
	"io"
	"sync"
)

type offsetReader struct {
	buf    io.ReadSeeker
	lock   sync.Mutex
	closed bool
}

func newOffsetReader(buf io.ReadSeeker, offset int64) (*offsetReader, error) {
	reader := &offsetReader{}
	_, err := buf.Seek(offset, io.SeekStart)
	if err != nil {
		return nil, err
	}

	reader.buf = buf
	return reader, nil
}

func (o *offsetReader) Close() error {
	o.lock.Lock()
	defer o.lock.Unlock()
	o.closed = true
	return nil
}

func (o *offsetReader) Read(p []byte) (int, error) {
	o.lock.Lock()
	defer o.lock.Unlock()
	if o.closed {
		return 0, io.EOF
	}
	return o.buf.Read(p)
}

func (o *offsetReader) Seek(offset int64, whence int) (int64, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.buf.Seek(offset, whence)
}

func (o *offsetReader) CloseAndCopy(offset int64) (*offsetReader, error) {
	if err := o.Close(); err != nil {
		return nil, err
	}

	return newOffsetReader(o.buf, offset)
}
