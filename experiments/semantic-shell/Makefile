.PHONY: fast check race verify benchmark

export GOWORK := off
export GOTOOLCHAIN := local
export GOFLAGS := -mod=vendor
export GOPROXY := off
export GOSUMDB := off

fast:
	go test -shuffle=on -count=1 ./...

check:
	go mod tidy -diff
	go vet ./...
	go test -shuffle=on -count=1 ./...

race:
	go test -race -shuffle=on -count=1 ./...

verify: check race
	go test -cover .
	go build -trimpath ./...

benchmark:
	go test -run=^$$ -bench=^Benchmark -benchmem -benchtime=500x -count=5 -cpu=1 .
