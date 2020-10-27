package local

import (
	"bytes"
	"fmt"
	"math/rand"
	"sync"

	chain "github.com/keep-network/keep-ecdsa/pkg/chain"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-common/pkg/subscription"
)

type localDeposit struct {
	keepAddress string
	pubkey      []byte
	digest      [32]byte
	signature   *Signature

	redemptionRequestedEvents []*chain.DepositRedemptionRequestedEvent
}

type Signature struct {
	V uint8
	R [32]uint8
	S [32]uint8
}

type localChainLogger struct {
	retrieveSignerPubkeyCalls       int
	provideRedemptionSignatureCalls int
}

func (lcl *localChainLogger) logRetrieveSignerPubkeyCall() {
	lcl.retrieveSignerPubkeyCalls++
}

func (lcl *localChainLogger) RetrieveSignerPubkeyCalls() int {
	return lcl.retrieveSignerPubkeyCalls
}

func (lcl *localChainLogger) logProvideRedemptionSignatureCall() {
	lcl.provideRedemptionSignatureCalls++
}

func (lcl *localChainLogger) ProvideRedemptionSignatureCalls() int {
	return lcl.provideRedemptionSignatureCalls
}

type TBTCLocalChain struct {
	*localChain

	mutex sync.Mutex

	logger *localChainLogger

	deposits                              map[string]*localDeposit
	depositCreatedHandlers                map[int]func(depositAddress string)
	depositRegisteredPubkeyHandlers       map[int]func(depositAddress string)
	depositRedemptionRequestedHandlers    map[int]func(depositAddress string)
	depositGotRedemptionSignatureHandlers map[int]func(depositAddress string)
	depositRedeemedHandlers               map[int]func(depositAddress string)
}

func NewTBTCLocalChain() *TBTCLocalChain {
	return &TBTCLocalChain{
		localChain:                            Connect().(*localChain),
		logger:                                &localChainLogger{},
		deposits:                              make(map[string]*localDeposit),
		depositCreatedHandlers:                make(map[int]func(depositAddress string)),
		depositRegisteredPubkeyHandlers:       make(map[int]func(depositAddress string)),
		depositRedemptionRequestedHandlers:    make(map[int]func(depositAddress string)),
		depositGotRedemptionSignatureHandlers: make(map[int]func(depositAddress string)),
		depositRedeemedHandlers:               make(map[int]func(depositAddress string)),
	}
}

func (tlc *TBTCLocalChain) CreateDeposit(depositAddress string) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	keepAddress := generateAddress()
	tlc.OpenKeep(keepAddress, []common.Address{
		generateAddress(),
		generateAddress(),
		generateAddress(),
	})

	tlc.deposits[depositAddress] = &localDeposit{
		keepAddress:               keepAddress.Hex(),
		redemptionRequestedEvents: make([]*chain.DepositRedemptionRequestedEvent, 0),
	}

	for _, handler := range tlc.depositCreatedHandlers {
		go func(handler func(depositAddress string), depositAddress string) {
			handler(depositAddress)
		}(handler, depositAddress)
	}
}

func (tlc *TBTCLocalChain) OnDepositCreated(
	handler func(depositAddress string),
) (subscription.EventSubscription, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	handlerID := generateHandlerID()

	tlc.depositCreatedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		tlc.mutex.Lock()
		defer tlc.mutex.Unlock()

		delete(tlc.depositCreatedHandlers, handlerID)
	}), nil
}

func (tlc *TBTCLocalChain) OnDepositRegisteredPubkey(
	handler func(depositAddress string),
) (subscription.EventSubscription, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	handlerID := generateHandlerID()

	tlc.depositRegisteredPubkeyHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		tlc.mutex.Lock()
		defer tlc.mutex.Unlock()

		delete(tlc.depositRegisteredPubkeyHandlers, handlerID)
	}), nil
}

func (tlc *TBTCLocalChain) RedeemDeposit(depositAddress string) error {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	if !bytes.Equal(deposit.digest[:], make([]byte, len(deposit.digest))) {
		return fmt.Errorf(
			"redemption of deposit [%v] already requested",
			depositAddress,
		)
	}

	var digest [32]byte
	rand.Read(digest[:])

	deposit.digest = digest

	err := tlc.requestSignature(
		common.HexToAddress(deposit.keepAddress),
		deposit.digest,
	)
	if err != nil {
		return err
	}

	for _, handler := range tlc.depositRedemptionRequestedHandlers {
		go func(handler func(depositAddress string), depositAddress string) {
			handler(depositAddress)
		}(handler, depositAddress)
	}

	deposit.redemptionRequestedEvents = append(
		deposit.redemptionRequestedEvents,
		&chain.DepositRedemptionRequestedEvent{
			DepositAddress:       depositAddress,
			Digest:               deposit.digest,
			UtxoValue:            nil,
			RedeemerOutputScript: nil,
			RequestedFee:         nil,
			Outpoint:             nil,
			BlockNumber:          0,
		},
	)

	return nil
}

func (tlc *TBTCLocalChain) OnDepositRedemptionRequested(
	handler func(depositAddress string),
) (subscription.EventSubscription, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	handlerID := generateHandlerID()

	tlc.depositRedemptionRequestedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		tlc.mutex.Lock()
		defer tlc.mutex.Unlock()

		delete(tlc.depositRedemptionRequestedHandlers, handlerID)
	}), nil
}

func (tlc *TBTCLocalChain) OnDepositGotRedemptionSignature(
	handler func(depositAddress string),
) (subscription.EventSubscription, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	handlerID := generateHandlerID()

	tlc.depositGotRedemptionSignatureHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		tlc.mutex.Lock()
		defer tlc.mutex.Unlock()

		delete(tlc.depositGotRedemptionSignatureHandlers, handlerID)
	}), nil
}

func (tlc *TBTCLocalChain) OnDepositRedeemed(
	handler func(depositAddress string),
) (subscription.EventSubscription, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	handlerID := generateHandlerID()

	tlc.depositRedeemedHandlers[handlerID] = handler

	return subscription.NewEventSubscription(func() {
		tlc.mutex.Lock()
		defer tlc.mutex.Unlock()

		delete(tlc.depositRedeemedHandlers, handlerID)
	}), nil
}

func (tlc *TBTCLocalChain) PastDepositRedemptionRequestedEvents(
	depositAddress string,
	startBlock uint64,
) ([]*chain.DepositRedemptionRequestedEvent, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return nil, fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	return deposit.redemptionRequestedEvents, nil
}

func (tlc *TBTCLocalChain) KeepAddress(depositAddress string) (string, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return "", fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	return deposit.keepAddress, nil
}

func (tlc *TBTCLocalChain) RetrieveSignerPubkey(depositAddress string) error {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	tlc.logger.logRetrieveSignerPubkeyCall()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	if len(deposit.pubkey) > 0 {
		return fmt.Errorf(
			"pubkey for deposit [%v] already retrieved",
			depositAddress,
		)
	}

	// lock upstream mutex to access `keeps` map safely
	tlc.handlerMutex.Lock()
	defer tlc.handlerMutex.Unlock()

	keep, ok := tlc.keeps[common.HexToAddress(deposit.keepAddress)]
	if !ok {
		return fmt.Errorf(
			"could not find keep for deposit [%v]",
			depositAddress,
		)
	}

	if len(keep.publicKey[:]) == 0 ||
		bytes.Equal(keep.publicKey[:], make([]byte, len(keep.publicKey))) {
		return fmt.Errorf(
			"keep of deposit [%v] doesn't have a public key yet",
			depositAddress,
		)
	}

	deposit.pubkey = keep.publicKey[:]

	for _, handler := range tlc.depositRegisteredPubkeyHandlers {
		go func(handler func(depositAddress string), depositAddress string) {
			handler(depositAddress)
		}(handler, depositAddress)
	}

	return nil
}

func (tlc *TBTCLocalChain) ProvideRedemptionSignature(
	depositAddress string,
	v uint8,
	r [32]uint8,
	s [32]uint8,
) error {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	tlc.logger.logProvideRedemptionSignatureCall()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	if deposit.signature != nil {
		return fmt.Errorf(
			"redemption signature for deposit [%v] already provided",
			depositAddress,
		)
	}

	deposit.signature = &Signature{
		V: v,
		R: r,
		S: s,
	}

	for _, handler := range tlc.depositGotRedemptionSignatureHandlers {
		go func(handler func(depositAddress string), depositAddress string) {
			handler(depositAddress)
		}(handler, depositAddress)
	}

	return nil
}

func (tlc *TBTCLocalChain) IncreaseRedemptionFee(
	depositAddress string,
	previousOutputValueBytes [8]uint8,
	newOutputValueBytes [8]uint8,
) error {
	panic("not implemented") // TODO: Implementation for unit testing purposes.
}

func (tlc *TBTCLocalChain) DepositPubkey(
	depositAddress string,
) ([]byte, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return nil, fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	if len(deposit.pubkey) == 0 {
		return nil, fmt.Errorf(
			"no pubkey for deposit [%v]",
			depositAddress,
		)
	}

	return deposit.pubkey, nil
}

func (tlc *TBTCLocalChain) DepositSignature(
	depositAddress string,
) (*Signature, error) {
	tlc.mutex.Lock()
	defer tlc.mutex.Unlock()

	deposit, ok := tlc.deposits[depositAddress]
	if !ok {
		return nil, fmt.Errorf("no deposit with address [%v]", depositAddress)
	}

	if deposit.signature == nil {
		return nil, fmt.Errorf(
			"no signature for deposit [%v]",
			depositAddress,
		)
	}

	return deposit.signature, nil
}

func (tlc *TBTCLocalChain) Logger() *localChainLogger {
	return tlc.logger
}
