package format

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewReader(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	input := strings.NewReader("test input")
	reader := NewReader(input)

	is.Equal(input, reader.Reader)
}

func TestReader_BeforeApply(t *testing.T) {
	t.Parallel()

	input := strings.NewReader("test input")

	tests := []struct {
		name         string
		reader       *Reader
		input        io.Reader
		expectReader io.Reader
	}{
		{
			name:         "sets reader from input",
			reader:       &Reader{},
			input:        input,
			expectReader: input,
		},
		{
			name:   "nil receiver does not panic",
			reader: nil,
			input:  input,
		},
		{
			name:         "nil input",
			reader:       &Reader{},
			input:        nil,
			expectReader: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			err := tt.reader.BeforeApply(tt.input)
			is.NoError(err)

			if tt.reader != nil {
				is.Equal(tt.expectReader, tt.reader.Reader)
			}
		})
	}
}

func TestReader_Read(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	input := strings.NewReader("hello")
	reader := NewReader(input)

	buf := make([]byte, 5)
	n, err := reader.Read(buf)

	is.NoError(err)
	is.Equal(5, n)
	is.Equal("hello", string(buf))
}

func TestReader_ImplementsIOReader(t *testing.T) {
	t.Parallel()

	var _ io.Reader = &Reader{}
	var _ io.Reader = Reader{}
}
