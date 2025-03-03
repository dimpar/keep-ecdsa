module github.com/keep-network/keep-ecdsa

go 1.16

replace (
	github.com/BurntSushi/toml => github.com/keep-network/toml v0.3.0
	github.com/blockcypher/gobcy => github.com/keep-network/gobcy v1.3.1
	github.com/btcsuite/btcd => github.com/keep-network/btcd v0.0.0-20190427004231-96897255fd17
	github.com/btcsuite/btcutil => github.com/keep-network/btcutil v0.0.0-20210527170813-e2ba6805a890
	github.com/urfave/cli => github.com/keep-network/cli v1.20.0
)

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/binance-chain/tss-lib v1.3.1
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v1.0.2
	github.com/celo-org/celo-blockchain v0.0.0-20210222234634-f8c8f6744526
	github.com/ethereum/go-ethereum v1.10.8
	github.com/gogo/protobuf v1.3.2
	github.com/google/go-cmp v0.5.4
	github.com/google/gofuzz v1.1.1-0.20200604201612-c04b05f3adfa
	github.com/ipfs/go-log v1.0.4
	github.com/keep-network/keep-common v1.5.1
	github.com/keep-network/keep-core v1.3.2-0.20210621132129-edc5dd03dad6
	github.com/keep-network/tbtc v1.1.1-0.20210319100902-7a3b5230ffcf
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.1
	gotest.tools/v3 v3.0.3
)
