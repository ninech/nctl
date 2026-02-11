package format

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewWriter(t *testing.T) {
	t.Parallel()
	is := require.New(t)

	buf := &bytes.Buffer{}
	writer := NewWriter(buf)

	is.Equal(buf, writer.Writer)
}

func TestWriter_BeforeApply(t *testing.T) {
	t.Parallel()

	buf := &bytes.Buffer{}

	tests := []struct {
		name         string
		writer       *Writer
		input        io.Writer
		expectWriter io.Writer
	}{
		{
			name:         "sets writer from input",
			writer:       &Writer{},
			input:        buf,
			expectWriter: buf,
		},
		{
			name:   "nil receiver does not panic",
			writer: nil,
			input:  buf,
		},
		{
			name:         "nil input",
			writer:       &Writer{},
			input:        nil,
			expectWriter: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			is := require.New(t)

			err := tt.writer.BeforeApply(tt.input)
			is.NoError(err)

			if tt.writer != nil {
				is.Equal(tt.expectWriter, tt.writer.Writer)
			}
		})
	}
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
			writer := NewWriter(buf)
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
			writer := NewWriter(buf)
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
			writer := NewWriter(output)
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
