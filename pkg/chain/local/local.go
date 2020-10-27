package local

import (
	"fmt"
	"math/big"
	"math/rand"
	"sync"
	"time"

	"github.com/keep-network/keep-core/pkg/chain/local"
	"github.com/keep-network/keep-ecdsa/pkg/utils/byteutils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/subscription"
	"github.com/keep-network/keep-core/pkg/chain"
	eth "github.com/keep-network/keep-ecdsa/pkg/chain"
	"github.com/keep-network/keep-ecdsa/pkg/ecdsa"
)

// Chain is an extention of eth.Handle interface which exposes
// additional functions useful for testing.
type Chain interface {
	eth.Handle

	OpenKeep(keepAddress common.Address, members []common.Address)
	CloseKeep(keepAddress common.Address) error
	TerminateKeep(keepAddress common.Address) error
	AuthorizeOperator(operatorAddress common.Address)
}

// localChain is an implementation of ethereum blockchain interface.
//
// It mocks the behaviour of a real blockchain, without the complexity of deployments,
// accounts, async transactions and so on. For use in tests ONLY.
type localChain struct {
	handlerMutex sync.Mutex

	blockCounter chain.BlockCounter

	keepAddresses []common.Address
	keeps         map[common.Address]*localKeep

	keepCreatedHandlers map[int]func(event *eth.BondedECDSAKeepCreatedEvent)

	clientAddress common.Address

	authorizations map[common.Address]bool
}

// Connect performs initialization for communication with Ethereum blockchain
// based on provided config.
func Connect() Chain {
	blockCounter, err := local.BlockCounter()
	if err != nil {
		panic(err) // should never happen
	}

	return &localChain{
		blockCounter:        blockCounter,
		keeps:               make(map[common.Address]*localKeep),
		keepCreatedHandlers: make(map[int]func(event *eth.BondedECDSAKeepCreatedEvent)),
		clientAddress:       common.HexToAddress("6299496199d99941193Fdd2d717ef585F431eA05"),
		authorizations:      make(map[common.Address]bool),
	}
}

func (lc *localChain) OpenKeep(keepAddress common.Address, members []common.Address) {
	err := lc.createKeepWithMembers(keepAddress, members)
	if err != nil {
		panic(err)
	}
}

func (lc *localChain) CloseKeep(keepAddress common.Address) error {
	return lc.closeKeep(keepAddress)
}

func (lc *localChain) TerminateKeep(keepAddress common.Address) error {
	return lc.terminateKeep(keepAddress)
}

func (lc *localChain) AuthorizeOperator(operator common.Address) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	lc.authorizations[operator] = true
}

// Address returns client's ethereum address.
func (lc *localChain) Address() common.Address {
	return lc.clientAddress
}

func (lc *localChain) StakeMonitor() (chain.StakeMonitor, error) {
	return nil, nil // not implemented.
}

// RegisterAsMemberCandidate registers client as a candidate to be selected
// to a keep.
func (lc *localChain) RegisterAsMemberCandidate(application common.Address) error {
	return nil
}

// OnBondedECDSAKeepCreated is a callback that is invoked when an on-chain
// notification of a new ECDSA keep creation is seen.
func (lc *localChain) OnBondedECDSAKeepCreated(
	handler func(event *eth.BondedECDSAKeepCreatedEvent),
) subscription.EventSubscription {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	handlerID := generateHandlerID()

	lc.keepCreatedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		lc.handlerMutex.Lock()
		defer lc.handlerMutex.Unlock()

		delete(lc.keepCreatedHandlers, handlerID)
	})
}

// OnSignatureRequested is a callback that is invoked on-chain
// when a keep's signature is requested.
func (lc *localChain) OnSignatureRequested(
	keepAddress common.Address,
	handler func(event *eth.SignatureRequestedEvent),
) (subscription.EventSubscription, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	handlerID := generateHandlerID()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return nil, fmt.Errorf(
			"failed to find keep with address: [%s]",
			keepAddress.String(),
		)
	}

	keep.signatureRequestedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		lc.handlerMutex.Lock()
		defer lc.handlerMutex.Unlock()

		delete(keep.signatureRequestedHandlers, handlerID)
	}), nil
}

// SubmitKeepPublicKey checks if public key has been already submitted for given
// keep address, if not it stores the key in a map.
func (lc *localChain) SubmitKeepPublicKey(
	keepAddress common.Address,
	publicKey [64]byte,
) error {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return fmt.Errorf(
			"failed to find keep with address: [%s]",
			keepAddress.String(),
		)
	}

	if keep.publicKey != [64]byte{} {
		return fmt.Errorf(
			"public key already submitted for keep [%s]",
			keepAddress.String(),
		)
	}

	keep.publicKey = publicKey

	return nil
}

// SubmitSignature submits a signature to a keep contract deployed under a
// given address.
func (lc *localChain) SubmitSignature(
	keepAddress common.Address,
	signature *ecdsa.Signature,
) error {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return fmt.Errorf(
			"failed to find keep with address: [%s]",
			keepAddress.String(),
		)
	}

	if keep.publicKey == [64]byte{} {
		return fmt.Errorf(
			"keep [%s] has no public key",
			keepAddress.String(),
		)
	}

	rBytes, err := byteutils.BytesTo32Byte(signature.R.Bytes())
	if err != nil {
		return err
	}

	sBytes, err := byteutils.BytesTo32Byte(signature.S.Bytes())
	if err != nil {
		return err
	}

	keep.signatureSubmittedEvents = append(
		keep.signatureSubmittedEvents,
		&eth.SignatureSubmittedEvent{
			Digest:     keep.latestDigest,
			R:          rBytes,
			S:          sBytes,
			RecoveryID: uint8(signature.RecoveryID),
		},
	)

	return nil
}

// IsAwaitingSignature checks if the keep is waiting for a signature to be
// calculated for the given digest.
func (lc *localChain) IsAwaitingSignature(
	keepAddress common.Address,
	digest [32]byte,
) (bool, error) {
	panic("implement")
}

// IsActive checks for current state of a keep on-chain.
func (lc *localChain) IsActive(keepAddress common.Address) (bool, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return false, fmt.Errorf("no keep with address [%v]", keepAddress)
	}

	return keep.status == active, nil
}

func (lc *localChain) BlockCounter() chain.BlockCounter {
	return lc.blockCounter
}

func (lc *localChain) IsRegisteredForApplication(application common.Address) (bool, error) {
	panic("implement")
}

func (lc *localChain) IsEligibleForApplication(application common.Address) (bool, error) {
	panic("implement")
}

func (lc *localChain) IsStatusUpToDateForApplication(application common.Address) (bool, error) {
	panic("implement")
}

func (lc *localChain) UpdateStatusForApplication(application common.Address) error {
	panic("implement")
}

func (lc *localChain) IsOperatorAuthorized(operator common.Address) (bool, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	return lc.authorizations[operator], nil
}

func (lc *localChain) GetKeepCount() (*big.Int, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	return big.NewInt(int64(len(lc.keeps))), nil
}

func (lc *localChain) GetKeepAtIndex(
	keepIndex *big.Int,
) (common.Address, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	index := int(keepIndex.Uint64())

	if index > len(lc.keepAddresses) {
		return common.HexToAddress("0x0"), fmt.Errorf("out of bounds")
	}

	return lc.keepAddresses[index], nil
}

func (lc *localChain) OnKeepClosed(
	keepAddress common.Address,
	handler func(event *eth.KeepClosedEvent),
) (subscription.EventSubscription, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	handlerID := generateHandlerID()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return nil, fmt.Errorf(
			"failed to find keep with address: [%s]",
			keepAddress.String(),
		)
	}

	keep.keepClosedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		lc.handlerMutex.Lock()
		defer lc.handlerMutex.Unlock()

		delete(keep.keepClosedHandlers, handlerID)
	}), nil
}

func (lc *localChain) OnKeepTerminated(
	keepAddress common.Address,
	handler func(event *eth.KeepTerminatedEvent),
) (subscription.EventSubscription, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	handlerID := generateHandlerID()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return nil, fmt.Errorf(
			"failed to find keep with address: [%s]",
			keepAddress.String(),
		)
	}

	keep.keepTerminatedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		lc.handlerMutex.Lock()
		defer lc.handlerMutex.Unlock()

		delete(keep.keepTerminatedHandlers, handlerID)
	}), nil
}

func (lc *localChain) OnConflictingPublicKeySubmitted(
	keepAddress common.Address,
	handler func(event *eth.ConflictingPublicKeySubmittedEvent),
) (subscription.EventSubscription, error) {
	panic("implement")
}

func (lc *localChain) OnPublicKeyPublished(
	keepAddress common.Address,
	handler func(event *eth.PublicKeyPublishedEvent),
) (subscription.EventSubscription, error) {
	panic("implement")
}

func (lc *localChain) LatestDigest(keepAddress common.Address) ([32]byte, error) {
	panic("implement")
}

func (lc *localChain) SignatureRequestedBlock(
	keepAddress common.Address,
	digest [32]byte,
) (uint64, error) {
	panic("implement")
}

func (lc *localChain) GetPublicKey(keepAddress common.Address) ([]uint8, error) {
	panic("implement")
}

func (lc *localChain) GetMembers(
	keepAddress common.Address,
) ([]common.Address, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	keep, ok := lc.keeps[keepAddress]
	if !ok {
		return nil, fmt.Errorf("no keep with address [%v]", keepAddress)
	}
	return keep.members, nil
}

func (lc *localChain) GetHonestThreshold(
	keepAddress common.Address,
) (uint64, error) {
	panic("implement")
}

func (lc *localChain) GetOpenedTimestamp(keepAddress common.Address) (time.Time, error) {
	panic("implement")
}

func (lc *localChain) PastSignatureSubmittedEvents(
	keepAddress string,
	startBlock uint64,
) ([]*eth.SignatureSubmittedEvent, error) {
	lc.handlerMutex.Lock()
	defer lc.handlerMutex.Unlock()

	keep, ok := lc.keeps[common.HexToAddress(keepAddress)]
	if !ok {
		return nil, fmt.Errorf("no keep with address [%v]", keepAddress)
	}

	return keep.signatureSubmittedEvents, nil
}

func (lc *localChain) BlockTimestamp(blockNumber *big.Int) (uint64, error) {
	panic("not implemented") // TODO: Implementation for unit testing purposes.
}

func generateHandlerID() int {
	// #nosec G404 (insecure random number source (rand))
	// Local chain implementation doesn't require secure randomness.
	return rand.Int()
}

func generateAddress() common.Address {
	var address [20]byte
	// #nosec G404 G104 (insecure random number source (rand) | error unhandled)
	// Local chain implementation doesn't require secure randomness.
	// Error can be ignored because according to the `rand.Read` docs it's
	// always `nil`.
	rand.Read(address[:])
	return address
}
