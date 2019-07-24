package ethereum

import (
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/keep-network/keep-tecdsa/pkg/chain/eth"
	"github.com/keep-network/keep-tecdsa/pkg/chain/eth/gen/abi"
)

// EthereumChain is an implementation of ethereum blockchain interface.
type EthereumChain struct {
	config                   *Config
	client                   *ethclient.Client
	transactorOptions        *bind.TransactOpts
	tecdsaKeepFactoryContract *abi.TECDSAKeepFactory
}

// Connect performs initialization for communication with Ethereum blockchain
// based on provided config.
func Connect(config *Config) (eth.Interface, error) {
	client, err := ethclient.Dial(config.URL)
	if err != nil {
		return nil, err
	}

	privateKey, err := crypto.HexToECDSA(config.PrivateKey)
	if err != nil {
		return nil, err
	}

	transactorOptions := bind.NewKeyedTransactor(privateKey)

	tecdsaKeepFactoryContractAddress, err := config.ContractAddress(TECDSAKeepFactoryContractName)
	if err != nil {
		return nil, err
	}
	tecdsaKeepFactoryContract, err := abi.NewTECDSAKeepFactory(
		tecdsaKeepFactoryContractAddress,
		client,
	)
	if err != nil {
		return nil, err
	}

	return &EthereumChain{
		config:                   config,
		client:                   client,
		transactorOptions:        transactorOptions,
		tecdsaKeepFactoryContract: tecdsaKeepFactoryContract,
	}, nil
}
