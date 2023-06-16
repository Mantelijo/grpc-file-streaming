package upload

import (
	"bytes"
	"io"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
)

func fillBytes(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return b
}
func TestFileByChunks(t *testing.T) {

	MB := 1024 * 1024
	tests := []struct {
		name     string
		file     *bytes.Buffer
		dest     *bytes.Buffer
		wantErr  string
		wantDest []byte
	}{
		{
			name:     "ok more than 4096B",
			file:     bytes.NewBuffer(fillBytes(10 * MB)),
			dest:     bytes.NewBuffer(nil),
			wantErr:  "",
			wantDest: fillBytes(10 * MB),
		},
		{
			name:     "ok less than 4096B",
			file:     bytes.NewBuffer(fillBytes(50)),
			dest:     bytes.NewBuffer(nil),
			wantErr:  "",
			wantDest: fillBytes(50),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := FileByChunks(tt.file, tt.dest)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.Equal(t, tt.wantDest, tt.dest.Bytes(), "expected equal site wantDest and dest")
			}

		})
	}
}

func TestPerformJsonMutation(t *testing.T) {
	tests := []struct {
		name     string
		in       io.Reader
		out      *bytes.Buffer
		wantErr  string
		wantDest []byte
	}{
		{
			name:    "read in error",
			in:      iotest.ErrReader(assert.AnError),
			out:     bytes.NewBuffer(nil),
			wantErr: "reading contents of file: assert.AnError general error for testing",
		},
		{
			name:    "invalid json",
			in:      bytes.NewBuffer([]byte(`gibberish invalid json`)),
			out:     bytes.NewBuffer(nil),
			wantErr: "unmarshalling file contents: invalid character 'g' looking for beginning of value",
		},
		{
			name: "ok, modifies json content 0",
			in: bytes.NewBuffer([]byte(`{
				"a_property":"aaaa",
				"b_property":12309123,
				"b_property_even":2000,
				"e_prop":231231
			}`)),
			out:      bytes.NewBuffer(nil),
			wantDest: []byte(`{"b_property":12309123,"b_property_even":3000}`),
		},
		{
			name: "ok, modifies json content 1",
			in: bytes.NewBuffer([]byte(`{
				"a_property":"aaaa",
				"b_property":12309123,
				"b_property_even":2000.5,
				"e_prop":231231
			}`)),
			out:      bytes.NewBuffer(nil),
			wantDest: []byte(`{"b_property":12309123,"b_property_even":2000.5}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := PerformJsonMutation(tt.in, tt.out)

			if tt.wantErr != "" {
				assert.EqualError(t, err, tt.wantErr)
			} else {
				assert.Equal(t, tt.wantDest, tt.out.Bytes(), "expected wantDest to match the contents of out")
			}

		})
	}
}
