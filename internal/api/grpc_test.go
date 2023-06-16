package api

import (
	"bytes"
	"context"
	"io"
	"net"
	"testing"

	"github.com/Mantelijo/grpc-file-stream/internal/gen"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

type fakeWriteCloser struct {
	bytesWritten uint64
	buff         []byte
	writeErr     error
	readErr      error
	seekErr      error
}

func (f *fakeWriteCloser) Write(p []byte) (n int, err error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.bytesWritten += uint64(len(p))
	f.buff = append(f.buff, p...)
	return len(p), nil
}

func (f *fakeWriteCloser) Close() error {
	return nil
}

func (f *fakeWriteCloser) Read(p []byte) (n int, err error) {
	return 0, f.readErr
}

func (f *fakeWriteCloser) Seek(offset int64, whence int) (int64, error) {
	return 0, f.seekErr
}

func dummyPayloadGenerator(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = 'a'
	}
	return b
}

func TestUploadFile(t *testing.T) {

	tests := []struct {
		name                      string
		writeCloserAndFilebackend func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error))
		uploadedFileMutator       func(in io.ReadCloser) error
		wantBytesWritten          uint64
		wantErr                   string
		wantResult                *gen.FileChunkResponse
		payload                   []byte
	}{
		{
			name: "create file error",
			writeCloserAndFilebackend: func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error)) {
				fwc := &fakeWriteCloser{}
				return fwc, func() (io.ReadWriteCloser, error) {
					return fwc, assert.AnError
				}
			},
			wantBytesWritten: 0,
			wantResult: &gen.FileChunkResponse{
				Message: "could not create upload destinaton",
				Status:  gen.FileChunkResponseStatus_error,
			},
			// 50 mb payload
			payload: dummyPayloadGenerator(50 * 1024 * 1024),
			uploadedFileMutator: func(in io.ReadCloser) error {
				return nil
			},
		},
		{
			name: "write file error",
			writeCloserAndFilebackend: func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error)) {
				fwc := &fakeWriteCloser{writeErr: assert.AnError}
				return fwc, func() (io.ReadWriteCloser, error) {
					return fwc, nil
				}
			},
			wantBytesWritten: 0,
			wantResult: &gen.FileChunkResponse{
				Message: "could not write to file",
				Status:  gen.FileChunkResponseStatus_error,
			},
			payload: dummyPayloadGenerator(50 * 1024 * 1024),
			uploadedFileMutator: func(in io.ReadCloser) error {
				return nil
			},
		},
		{
			name: "seek file error",
			writeCloserAndFilebackend: func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error)) {
				fwc := &fakeWriteCloser{seekErr: assert.AnError}
				return fwc, func() (io.ReadWriteCloser, error) {
					return fwc, nil
				}
			},
			wantBytesWritten: 50 * 1024 * 1024,
			wantResult: &gen.FileChunkResponse{
				Message: "internal server error",
				Status:  gen.FileChunkResponseStatus_error,
			},
			payload: dummyPayloadGenerator(50 * 1024 * 1024),
			uploadedFileMutator: func(in io.ReadCloser) error {
				return nil
			},
		},
		{
			name: "ok",
			writeCloserAndFilebackend: func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error)) {
				fwc := &fakeWriteCloser{}
				return fwc, func() (io.ReadWriteCloser, error) {
					return fwc, nil
				}
			},
			wantBytesWritten: 50 * 1024 * 1024,
			wantResult: &gen.FileChunkResponse{
				Message: "upload completed",
				Status:  gen.FileChunkResponseStatus_ok,
			},
			payload: dummyPayloadGenerator(50 * 1024 * 1024),
			uploadedFileMutator: func(in io.ReadCloser) error {
				return nil
			},
		},
		{
			name: "mutating json error",
			writeCloserAndFilebackend: func() (*fakeWriteCloser, func() (io.ReadWriteCloser, error)) {
				fwc := &fakeWriteCloser{}
				return fwc, func() (io.ReadWriteCloser, error) {
					return fwc, nil
				}
			},
			wantBytesWritten: 50 * 1024 * 1024,
			wantResult: &gen.FileChunkResponse{
				Message: "could not mutate json content",
				Status:  gen.FileChunkResponseStatus_error,
			},
			payload: dummyPayloadGenerator(50 * 1024 * 1024),
			uploadedFileMutator: func(in io.ReadCloser) error {
				return assert.AnError
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// fake writer closer
			fwc, fileBackend := tt.writeCloserAndFilebackend()

			// 4MB
			lis := bufconn.Listen(4096 * 1024)
			s := grpc.NewServer()
			gen.RegisterFileUploaderServer(s, &GRPCServer{
				l:                   zap.NewNop(),
				fileBackend:         fileBackend,
				uploadedFileMutator: tt.uploadedFileMutator,
			})

			// Start the server goroutine
			go func() {
				if err := s.Serve(lis); err != nil {
					t.Fatalf("unexpected server error: %v", err)
				}
			}()

			ctx := context.Background()
			conn, err := grpc.DialContext(
				ctx,
				"testnet-bufconn",
				grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
					return lis.Dial()
				}),
				grpc.WithTransportCredentials(insecure.NewCredentials()),
			)
			if err != nil {
				t.Fatalf("could not dial bufconn: %v", err)
			}
			defer conn.Close()

			client := gen.NewFileUploaderClient(conn)
			stream, err := client.UploadFile(context.Background())
			assert.NoError(t, err)

			sentBytes := 0
			// Emulate file upload at 2 mb chunks
			r := bytes.NewReader(tt.payload)
			for {
				buf := make([]byte, 2*1024*1024)
				n, err := r.Read(buf)
				if err == io.EOF {
					break
				}
				assert.NoError(t, err)
				if n <= 0 {
					break
				}
				err = stream.Send(&gen.FileChunk{
					Data: buf,
				})
				sentBytes += n

				if err != nil {
					// Expect only EOF here since sends from client should go
					// through
					assert.EqualError(t, err, io.EOF.Error())
				}
			}

			resp, err := stream.CloseAndRecv()
			assert.NoError(t, err)

			// Wait for server to respond and stop
			s.GracefulStop()

			// Expect written bytes to match
			assert.EqualValues(t, tt.wantBytesWritten, fwc.bytesWritten)

			// Expect responses to match
			if tt.wantResult != nil {
				assert.Equal(t, tt.wantResult.Message, resp.Message)
				assert.Equal(t, tt.wantResult.Status, resp.Status)
			}
		})
	}

}
