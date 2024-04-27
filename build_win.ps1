$env:CGO_ENABLED=0; $env:GOOS="windows"; $env:GOARCH="amd64"; go build -o camcast.exe -ldflags="-H=windowsgui -s -w" -trimpath -tags netgo ./src
