package format

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWriter(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	out, errOut := &bytes.Buffer{}, &bytes.Buffer{}
	writer := NewWriter(out, errOut)

	is.Equal(out, writer.Out())
	is.Equal(errOut, writer.Err())
}

func TestWriter_BeforeApply(t *testing.T) {
	t.Parallel()

	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}

	tests := []struct {
		name         string
		writer       *Writer
		streams      IOStreams
		expectWriter io.Writer
		expectErr    io.Writer
	}{
		{
			name:         "sets writers from the streams",
			writer:       &Writer{},
			streams:      IOStreams{Out: stdout, ErrOut: stderr},
			expectWriter: stdout,
			expectErr:    stderr,
		},
		{
			name:    "nil receiver does not panic",
			writer:  nil,
			streams: IOStreams{Out: stdout, ErrOut: stderr},
		},
		{
			name:         "empty streams",
			writer:       &Writer{},
			expectWriter: nil,
			expectErr:    io.Discard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			err := tt.writer.BeforeApply(tt.streams)
			is.NoError(err)

			if tt.writer != nil {
				is.Equal(tt.expectWriter, tt.writer.Writer)
				is.Equal(tt.expectErr, tt.writer.Err())
			}
		})
	}
}

func TestWriter_Err(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	var nilWriter *Writer
	is.Equal(io.Discard, nilWriter.Err())
	is.Equal(io.Discard, (&Writer{}).Err())
}

func TestWriter_Streams(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	_, stdout := pipe(t)
	_, stderr := pipe(t)
	w := &Writer{}
	is.NoError(w.BeforeApply(IOStreams{Out: stdout, ErrOut: stderr}))

	// Only the files themselves expose their file descriptors.
	is.Same(stdout, w.Out())
	is.Same(stderr, w.Err())

	var nilWriter *Writer
	is.Nil(nilWriter.Out())
	is.Nil((&Writer{}).Out())
}

// pipe returns both ends of a pipe, [os.File]s closed at the end of the test.
func pipe(t *testing.T) (r, w *os.File) {
	t.Helper()

	r, w, err := os.Pipe()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	return r, w
}

func TestWriter_writer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		writer *Writer
		expect io.Writer
	}{
		{
			name:   "nil Writer field returns io.Discard",
			writer: &Writer{},
			expect: io.Discard,
		},
		{
			name:   "nil receiver returns io.Discard",
			writer: nil,
			expect: io.Discard,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			w := tt.writer.writer()
			is.Equal(tt.expect, w)
		})
	}
}

func TestWriter_Write(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		writer   func(io.Writer) Writer
		expected string
	}{
		{
			name:     "writes to the wrapped writer",
			writer:   func(w io.Writer) Writer { return NewWriter(w, io.Discard) },
			expected: "hello",
		},
		{
			name:   "zero value discards instead of panicking",
			writer: func(io.Writer) Writer { return Writer{} },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			buf := &bytes.Buffer{}
			n, err := fmt.Fprint(tt.writer(buf), "hello")

			is.NoError(err)
			is.Equal(len("hello"), n)
			is.Equal(tt.expected, buf.String())
		})
	}
}

func TestWriter_Print(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   func(Writer)
		expected string
	}{
		{
			name:     "Printf",
			action:   func(w Writer) { w.Printf("hello %s", "world") },
			expected: "hello world",
		},
		{
			name:     "Println",
			action:   func(w Writer) { w.Println("hello", "world") },
			expected: "hello world\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			buf := &bytes.Buffer{}
			writer := NewWriter(buf, buf)
			tt.action(writer)

			is.Equal(tt.expected, buf.String())
		})
	}
}

func TestWriter_FormattedOutput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		action   func(Writer)
		contains []string
	}{
		{
			name:     "Successf",
			action:   func(w Writer) { w.Successf("🎉", "created %s", "resource") },
			contains: []string{SuccessChar, "created resource", "🎉"},
		},
		{
			name:     "Success",
			action:   func(w Writer) { w.Success("🎉", "operation complete") },
			contains: []string{SuccessChar, "operation complete", "🎉"},
		},
		{
			name:     "Failuref",
			action:   func(w Writer) { w.Failuref("💥", "failed to %s", "connect") },
			contains: []string{FailureChar, "failed to connect", "💥"},
		},
		{
			name:     "Warningf",
			action:   func(w Writer) { w.Warningf("something %s", "happened") },
			contains: []string{"Warning:", "something happened"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			buf := &bytes.Buffer{}
			writer := NewWriter(buf, buf)
			tt.action(writer)

			for _, s := range tt.contains {
				is.Contains(buf.String(), s)
			}
		})
	}
}

func TestWriter_ImplementsIOWriter(t *testing.T) {
	t.Parallel()

	var _ io.Writer = &Writer{}
	var _ io.Writer = Writer{}
}

func TestConfirm(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		expected  bool
		expectErr bool
	}{
		{"lowercase y", "y\n", true, false},
		{"uppercase Y", "Y\n", true, false},
		{"lowercase yes", "yes\n", true, false},
		{"uppercase YES", "YES\n", true, false},
		{"mixed case Yes", "Yes\n", true, false},
		{"lowercase n", "n\n", false, false},
		{"uppercase N", "N\n", false, false},
		{"lowercase no", "no\n", false, false},
		{"uppercase NO", "NO\n", false, false},
		{"other input", "maybe\n", false, false},
		{"empty input", "", false, true},
		{"whitespace only", "   \n", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			output := &bytes.Buffer{}
			writer := NewWriter(output, output)
			reader := NewReader(strings.NewReader(tt.input))

			result, err := writer.Confirm(reader, "Continue?")

			if tt.expectErr {
				is.Error(err)
				return
			}

			is.NoError(err)
			is.Equal(tt.expected, result)
			is.Contains(output.String(), "Continue? [y|n]:")
		})
	}
}
