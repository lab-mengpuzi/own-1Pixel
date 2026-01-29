# README.md

## Deploy

Windows 10 -> version

```plain
golang-windows 1.25.0
```

Windows 10 -> run

```powershell
C:\own-1Pixel># Install Dependency
C:\own-1Pixel>go mod tidy
C:\own-1Pixel>
C:\own-1Pixel># Run main.go
C:\own-1Pixel>go run main.go
```

Windows 10 -> build

```powershell
C:\own-1Pixel># Build Windows / Linux / Darwin
C:\own-1Pixel># GOARCH amd64 / arm64
C:\own-1Pixel>$GOARCH = "amd64"; $env:GOARCH=$GOARCH; $env:GOOS="windows"; go build -ldflags='-s -w' -o "own-1Pixel.windows-$GOARCH.exe"; Write-Host "Build own-1Pixel.windows-$GOARCH.exe SUCCESS!"; $env:GOOS="linux"; go build -ldflags='-s -w' -o "own-1Pixel.linux-$GOARCH"; Write-Host "Build own-1Pixel.linux-$GOARCH SUCCESS!"; $env:GOOS="darwin"; go build -ldflags='-s -w' -o "own-1Pixel.darwin-$GOARCH"; Write-Host "Build own-1Pixel.darwin-$GOARCH SUCCESS!"
```