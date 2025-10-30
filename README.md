# README.md

## Deploy

Windows 10 -> version

```plain
golang-windows 1.21.0
```

Windows 10 -> run

```powershell
C:\Users\Administrator\Downloads\own-1Pixel># Install Dependency
C:\Users\Administrator\Downloads\own-1Pixel>go mod tidy
C:\Users\Administrator\Downloads\own-1Pixel>
C:\Users\Administrator\Downloads\own-1Pixel># Run main.go
C:\Users\Administrator\Downloads\own-1Pixel>go run main.go
```

Windows 10 -> build

```powershell
C:\Users\Administrator\Downloads\own-1Pixel># Windows build
C:\Users\Administrator\Downloads\own-1Pixel>go build -o own-1Pixel.exe main.go
C:\Users\Administrator\Downloads\own-1Pixel>
C:\Users\Administrator\Downloads\own-1Pixel># Linux build
C:\Users\Administrator\Downloads\own-1Pixel>set GOOS=linux ; set GOARCH=amd64 ; go build -o own-1Pixel main.go
C:\Users\Administrator\Downloads\own-1Pixel>
C:\Users\Administrator\Downloads\own-1Pixel># Production mode (Remove debug symbol)
C:\Users\Administrator\Downloads\own-1Pixel>go build -ldflags='-s -w' -o own-1Pixel.exe main.go
```