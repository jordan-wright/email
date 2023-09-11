package email

import "io"

// ChunkedReader provides reading by specified portion size.
// Only last chunk can be lesser or zero-size same time with EOF or other error
type ChunkedReader struct {
	r        io.Reader
	chunkLen int
}

func (cr *ChunkedReader) Read(b []byte) (int, error) {
	accumulatedBytes := 0
	if len(b) < cr.chunkLen {
		return 0, io.ErrShortBuffer
	}
	var err error
	var n int
	for accumulatedBytes < cr.chunkLen && err == nil {
		n, err = cr.r.Read(b[accumulatedBytes:])
		accumulatedBytes += n
	}
	return accumulatedBytes, err
}

func NewChunkedReader(r io.Reader, chunkLen int) *ChunkedReader {
	return &ChunkedReader{
		r:        r,
		chunkLen: chunkLen,
	}
}
