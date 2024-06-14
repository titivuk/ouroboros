## Test

```
go test -v ./...
```

## Build

```
env GOOS=linux GOARCH=amd64 go build -o orb
```

## Usage

```
orb go run main.go
orb node index.js
orb npx ts-node index.ts
```

See examples folder for sample programs

## Ignore directories
create `.rignore` file and add directories / files you would like to ignore

Note: the implementation is super basic and not finished so only exact paths are supported

## Dependencies

It requires `Inotify` to work