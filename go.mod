module github.com/celo-org/eksportisto

go 1.13

require (
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/celo-org/kliento v0.0.0-20200428005346-e515c2c18075
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.4.2-0.20180625184442-8e610b2b55bf
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/felixge/httpsnoop v1.0.1
	github.com/google/addlicense v0.0.0-20200422172452-68a83edd47bc // indirect
	github.com/gopherjs/gopherjs v0.0.0-20181017120253-0766667cb4d1
	github.com/gorilla/mux v1.7.4
	github.com/influxdata/influxdb v1.2.3-0.20180221223340-01288bdb0883
	github.com/jsternberg/zap-logfmt v1.2.0 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/prometheus/client_golang v1.5.1
	github.com/prometheus/tsdb v0.7.1 // indirect
	github.com/stretchr/testify v1.5.1 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
)

replace github.com/celo-org/bls-zexe/go => ./external/bls-zexe/go

// Use this to use external build
replace github.com/ethereum/go-ethereum => github.com/celo-org/celo-blockchain v0.0.0-20200422165556-dee2db520e23
