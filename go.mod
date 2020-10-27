module github.com/celo-org/eksportisto

go 1.13

require (
	github.com/Nvveen/Gotty v0.0.0-20120604004816-cd527374f1e5 // indirect
	github.com/celo-org/kliento v0.1.2-0.20200608140637-c5afc8cf0f44
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/felixge/httpsnoop v1.0.1
	github.com/google/addlicense v0.0.0-20200422172452-68a83edd47bc // indirect
	github.com/gorilla/mux v1.7.4
	github.com/jsternberg/zap-logfmt v1.2.0 // indirect
	github.com/mattn/go-sqlite3 v2.0.3+incompatible
	github.com/onsi/gomega v1.9.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/prometheus/client_golang v1.5.1
	github.com/stretchr/testify v1.5.1 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
)

// Use this to use external build
replace github.com/ethereum/go-ethereum => github.com/celo-org/celo-blockchain v0.0.0-20200519153823-adbdc7f8c27e
