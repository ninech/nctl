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

// BeforeApply ensures that Kong initializes the [Reader].
func (r *Reader) BeforeApply(reader io.Reader) error {
	if r != nil && reader != nil {
		r.Reader = reader
	}

	return nil
}
