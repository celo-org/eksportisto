module github.com/celo-org/eksportisto

go 1.13

require (
	github.com/celo-org/kliento v0.0.0-20200428005346-e515c2c18075
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/ethereum/go-ethereum v1.9.13
	github.com/felixge/httpsnoop v1.0.1
	github.com/google/addlicense v0.0.0-20200422172452-68a83edd47bc // indirect
	github.com/gorilla/mux v1.7.4
	github.com/prometheus/client_golang v1.5.1
	github.com/stretchr/testify v1.5.1 // indirect
	golang.org/x/sync v0.0.0-20200317015054-43a5402ce75a
)

replace github.com/celo-org/bls-zexe/go => ./external/bls-zexe/go

// Use this to use external build
replace github.com/ethereum/go-ethereum => github.com/celo-org/celo-blockchain v0.0.0-20200422165556-dee2db520e23
