CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o camcast -ldflags="-s -w" -trimpath -tags netgo ./src
