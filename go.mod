module github.com/celo-org/eksportisto

go 1.13

require (
	cloud.google.com/go/bigquery v1.8.0
	github.com/celo-org/celo-blockchain v1.3.2
	github.com/celo-org/kliento v0.2.1-0.20220118184311-83bd0da5cb6c
	github.com/cheekybits/is v0.0.0-20150225183255-68e9c0620927 // indirect
	github.com/felixge/httpsnoop v1.0.1
	github.com/go-errors/errors v1.4.0
	github.com/go-redis/redis/v8 v8.11.3
	github.com/gorilla/mux v1.7.4
	github.com/matryer/try v0.0.0-20161228173917-9ac251b645a2 // indirect
	github.com/neilotoole/errgroup v0.1.5
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/common v0.9.1
	github.com/spf13/cobra v1.2.1
	github.com/spf13/viper v1.8.1
	golang.org/x/sync v0.0.0-20210220032951-036812b2e83c
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e
)

// DO NOT CHANGE DIRECTORY: Create a symlink so this works
// replace github.com/celo-org/celo-blockchain => ../celo-blockchain

// replace github.com/celo-org/kliento => ../kliento
