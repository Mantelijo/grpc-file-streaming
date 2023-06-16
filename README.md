# gRPC file upload service

To run the service
```bash
go run cmd/main.go
```

Clients can upload files by streaming `FileChunk`s. Server accepts default size
payloads (4MB). Uploaded file will be placed in a current working directory.
Uploaded file will be named `uploaded-file-<timestamp>`. If uploaded contents
contain a valid JSON structure, it will be mutated and written o a file named
`uploaded-file-mutated-<timestap>` in cwd.





