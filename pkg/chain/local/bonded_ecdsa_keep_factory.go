package local

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	eth "github.com/keep-network/keep-ecdsa/pkg/chain"
)

func (c *localChain) createKeep(keepAddress common.Address) error {
	return c.createKeepWithMembers(keepAddress, []common.Address{})
}

func (c *localChain) createKeepWithMembers(
	keepAddress common.Address,
	members []common.Address,
) error {
	c.handlerMutex.Lock()
	defer c.handlerMutex.Unlock()

	if _, ok := c.keeps[keepAddress]; ok {
		return fmt.Errorf(
			"keep already exists for address [%s]",
			keepAddress.String(),
		)
	}

	localKeep := &localKeep{
		publicKey:                  [64]byte{},
		members:                    members,
		signatureRequestedHandlers: make(map[int]func(event *eth.SignatureRequestedEvent)),
		keepClosedHandlers:         make(map[int]func(event *eth.KeepClosedEvent)),
		keepTerminatedHandlers:     make(map[int]func(event *eth.KeepTerminatedEvent)),
	}

	c.keeps[keepAddress] = localKeep
	c.keepAddresses = append(c.keepAddresses, keepAddress)

	keepCreatedEvent := &eth.BondedECDSAKeepCreatedEvent{
		KeepAddress: keepAddress,
	}

	for _, handler := range c.keepCreatedHandlers {
		go func(
			handler func(event *eth.BondedECDSAKeepCreatedEvent),
			keepCreatedEvent *eth.BondedECDSAKeepCreatedEvent,
		) {
			handler(keepCreatedEvent)
		}(handler, keepCreatedEvent)
	}

	return nil
}
