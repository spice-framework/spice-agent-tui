# Deterministic benchmark evidence

`go test -run '^$' -bench . -benchmem -count 5 .` measures public semantic
snapshot projection, command parsing, and JSONL record emission. Inputs use
fixed values and in-memory streams; no daemon, terminal, clock value, random
source, or network participates.

The experiment does not set a promotion budget yet. Its first green phase
boundary records a reproducible baseline; later changes must report median
allocations and latency with the exact commit and Go 1.26.5 environment.

Initial Windows/amd64 baseline on Go 1.26.5 (Ryzen 9 5900X, 500 iterations,
five samples, one CPU):

| Benchmark | Median | Bytes/op | Allocs/op |
| --- | ---: | ---: | ---: |
| `BenchmarkApplyActivityUpdate` | 823.8 ns/op | 80 | 7 |
| `BenchmarkEncodeSemanticView` | 1,185 ns/op | 1,088 | 6 |
