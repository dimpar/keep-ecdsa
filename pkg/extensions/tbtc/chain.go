package tbtc

import (
	"github.com/keep-network/keep-common/pkg/subscription"
	chain "github.com/keep-network/keep-ecdsa/pkg/chain"
)

// Handle represents a chain handle extended with TBTC-specific capabilities.
type Handle interface {
	chain.Handle

	Deposit
	TBTCSystem
}

// Deposit is an interface that provides ability to interact
// with Deposit contracts.
type Deposit interface {
	// KeepAddress returns the underlying keep address for the
	// provided deposit.
	KeepAddress(depositAddress string) (string, error)

	// RetrieveSignerPubkey retrieves the signer public key for the
	// provided deposit.
	RetrieveSignerPubkey(depositAddress string) error

	// ProvideRedemptionSignature provides the redemption signature for the
	// provided deposit.
	ProvideRedemptionSignature(
		depositAddress string,
		v uint8,
		r [32]uint8,
		s [32]uint8,
	) error

	// IncreaseRedemptionFee increases the redemption fee for the
	// provided deposit.
	IncreaseRedemptionFee(
		depositAddress string,
		previousOutputValueBytes [8]uint8,
		newOutputValueBytes [8]uint8,
	) error
}

// TBTCSystem is an interface that provides ability to interact
// with TBTCSystem contract.
type TBTCSystem interface {
	// OnDepositCreated installs a callback that is invoked when an
	// on-chain notification of a new deposit creation is seen.
	OnDepositCreated(
		handler func(depositAddress string),
	) (subscription.EventSubscription, error)

	// OnDepositRegisteredPubkey installs a callback that is invoked when an
	// on-chain notification of a deposit's pubkey registration is seen.
	OnDepositRegisteredPubkey(
		handler func(depositAddress string),
	) (subscription.EventSubscription, error)

	// OnDepositRedemptionRequested installs a callback that is invoked when an
	// on-chain notification of a deposit redemption request is seen.
	OnDepositRedemptionRequested(
		handler func(depositAddress string),
	) (subscription.EventSubscription, error)

	// OnDepositGotRedemptionSignature installs a callback that is invoked
	// when an on-chain notification of a deposit receiving a redemption
	// signature is seen.
	OnDepositGotRedemptionSignature(
		handler func(depositAddress string),
	) (subscription.EventSubscription, error)

	// OnDepositRedeemed installs a callback that is invoked when an
	// on-chain notification of a deposit redemption is seen.
	OnDepositRedeemed(
		handler func(depositAddress string),
	) (subscription.EventSubscription, error)

	// PastDepositRedemptionRequestedEvents returns all redemption requested
	// events for the given deposit which occurred after the provided start block.
	// All implementations should returns those events sorted by the
	// block number in the ascending order.
	PastDepositRedemptionRequestedEvents(
		depositAddress string,
		startBlock uint64,
	) ([]*chain.DepositRedemptionRequestedEvent, error)
}
