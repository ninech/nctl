package format

import "io"

// Reader is a wrapper around an [io.Reader].
type Reader struct {
	io.Reader
}

// NewReader returns a new [Reader].
func NewReader(r io.Reader) Reader {
	return Reader{Reader: r}
}

// In returns the stream the Reader reads from.
// It is the stream itself rather than a wrapper,
// so that a terminal stays detectable and a subprocess inherits it,
// unlike the Reader, which never is a terminal.
// It is nil if the receiver or its stream is nil.
func (r *Reader) In() io.Reader {
	if r == nil {
		return nil
	}

	return r.Reader
}

// reader returns the underlying reader, or an empty reader if the receiver or
// the wrapped [io.Reader] is nil.
func (r *Reader) reader() io.Reader {
	if r == nil || r.Reader == nil {
		return eofReader{}
	}

	return r.Reader
}

// Read implements [io.Reader]. It shadows the promoted method of the embedded
// [io.Reader] so that reading from an uninitialized Reader returns [io.EOF]
// instead of panicking. The receiver is a value so that Reader keeps
// implementing [io.Reader] when passed around by value.
func (r Reader) Read(p []byte) (int, error) {
	return r.reader().Read(p)
}

// eofReader is an [io.Reader] which is always exhausted.
type eofReader struct{}

// Read always returns [io.EOF].
func (eofReader) Read([]byte) (int, error) {
	return 0, io.EOF
}

// BeforeApply ensures that Kong initializes the [Reader]
// with the input of the bound [IOStreams].
// It is subject to the same export requirement as [Writer.BeforeApply].
func (r *Reader) BeforeApply(streams IOStreams) error {
	if r != nil && streams.In != nil {
		r.Reader = streams.In
	}

	return nil
}
