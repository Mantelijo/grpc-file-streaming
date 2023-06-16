package api

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/Mantelijo/grpc-file-stream/internal/gen"
	"github.com/Mantelijo/grpc-file-stream/internal/upload"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// StartGRPCServer starts the gRPC server on given address
func StartGRPCServer(address string, l *zap.Logger) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen to tcp at %s: %w", address, err)
	}
	grpcServer := grpc.NewServer()
	gen.RegisterFileUploaderServer(grpcServer, NewFileUploaderService(l))

	// Enable refleciton for local testing
	reflection.Register(grpcServer)

	l.Info("starting the gRPC server", zap.String("address", address))

	return grpcServer.Serve(listener)
}

func NewFileUploaderService(l *zap.Logger) gen.FileUploaderServer {
	return &GRPCServer{
		l:                   l,
		fileBackend:         defaultFileBackend,
		uploadedFileMutator: defaultUploadedFileMutator,
	}
}

func defaultFileBackend() (io.ReadWriteCloser, error) {
	timeSuffix := time.Now().Format("1136239445")
	f, err := os.OpenFile("uploaded-file-"+timeSuffix, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return nil, fmt.Errorf("could not create a new file: %w", err)
	}
	return f, nil
}

func defaultUploadedFileMutator(in io.ReadCloser) error {
	timeSuffix := time.Now().Format("1136239445")
	f, err := os.OpenFile("uploaded-file-mutated"+timeSuffix, os.O_TRUNC|os.O_RDWR|os.O_CREATE, 0660)
	if err != nil {
		return fmt.Errorf("could not create a new file: %w", err)
	}
	err = upload.PerformJsonMutation(in, f)
	// On error - remove the "mutated" file
	if err != nil {
		os.Remove(f.Name())
		return err
	}
	return nil
}

var _ (gen.FileUploaderServer) = (*GRPCServer)(nil)

type GRPCServer struct {
	gen.UnimplementedFileUploaderServer
	l *zap.Logger

	// fileBackend constructs a target file for upload contents
	fileBackend func() (io.ReadWriteCloser, error)

	// uploadedFileMutator validates that given in contents is valid json and
	// performs a mutation
	uploadedFileMutator func(in io.ReadCloser) error
}

func (g *GRPCServer) UploadFile(stream gen.FileUploader_UploadFileServer) error {
	f, err := g.fileBackend()
	if err != nil {
		g.l.Error("creating new destination file", zap.Error(err))

		return stream.SendAndClose(&gen.FileChunkResponse{
			Message: "could not create upload destinaton",
			Status:  gen.FileChunkResponseStatus_error,
		})
	}
	defer f.Close()

	for {
		chunk, err := stream.Recv()
		// Stream completed
		if err == io.EOF {
			break
		}

		if err != nil {
			return stream.SendAndClose(&gen.FileChunkResponse{
				Message: "error while reading stream",
				Status:  gen.FileChunkResponseStatus_error,
			})
		}

		// Process the chunk
		err = upload.FileByChunks(
			bytes.NewBuffer(chunk.Data),
			f,
		)

		if err != nil {
			g.l.Error("uploading file chunks", zap.Error(err))
			return stream.SendAndClose(&gen.FileChunkResponse{
				Message: "could not write to file",
				Status:  gen.FileChunkResponseStatus_error,
			})
		}

	}

	// Reset internal file pointer whenever possible
	seeker, ok := f.(io.Seeker)
	if ok {
		_, err := seeker.Seek(0, 0)
		if err != nil {
			g.l.Error("resetting file pointer", zap.Error(err))
			return stream.SendAndClose(&gen.FileChunkResponse{
				Message: "internal server error",
				Status:  gen.FileChunkResponseStatus_error,
			})
		}
	}

	// Perform json mutation
	err = g.uploadedFileMutator(f)
	if err != nil {
		g.l.Error("mutating json content of the uploaded file", zap.Error(err))
		return stream.SendAndClose(&gen.FileChunkResponse{
			Message: "could not mutate json content",
			Status:  gen.FileChunkResponseStatus_error,
		})
	}

	// Upload complete - send the response to client
	return stream.SendAndClose(&gen.FileChunkResponse{
		Message: "upload completed",
		Status:  gen.FileChunkResponseStatus_ok,
	})
}
