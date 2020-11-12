package tbtc

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/keep-network/keep-ecdsa/pkg/ecdsa"
	"github.com/keep-network/keep-ecdsa/pkg/utils/byteutils"

	"github.com/ethereum/go-ethereum/common"
	"github.com/keep-network/keep-ecdsa/pkg/chain/local"
)

const (
	timeout        = 500 * time.Millisecond
	depositAddress = "0xa5FA806723A7c7c8523F33c39686f20b52612877"
)

func TestRetrievePubkey_TimeoutElapsed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	keepPubkey, err := submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 1
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}

	depositPubkey, err := tbtcChain.DepositPubkey(depositAddress)
	if err != nil {
		t.Errorf(
			"unexpected error while fetching deposit pubkey: [%v]",
			err,
		)
	}

	if !bytes.Equal(keepPubkey[:], depositPubkey) {
		t.Errorf(
			"unexpected public key\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			keepPubkey,
			depositPubkey,
		)
	}
}

func TestRetrievePubkey_StopEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	keepPubkey, err := submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the stop event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// invoke the action which will trigger the stop event in result
	err = tbtcChain.RetrieveSignerPubkey(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 1
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}

	depositPubkey, err := tbtcChain.DepositPubkey(depositAddress)
	if err != nil {
		t.Errorf(
			"unexpected error while fetching deposit pubkey: [%v]",
			err,
		)
	}

	if !bytes.Equal(keepPubkey[:], depositPubkey) {
		t.Errorf(
			"unexpected public key\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			keepPubkey,
			depositPubkey,
		)
	}
}

func TestRetrievePubkey_KeepClosedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep closed event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = closeKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 0
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}

	_, err = tbtcChain.DepositPubkey(depositAddress)

	expectedError := fmt.Errorf(
		"no pubkey for deposit [%v]",
		depositAddress,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedError,
			err,
		)
	}
}

func TestRetrievePubkey_KeepTerminatedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep terminated event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = terminateKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 0
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}

	_, err = tbtcChain.DepositPubkey(depositAddress)

	expectedError := fmt.Errorf(
		"no pubkey for deposit [%v]",
		depositAddress,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedError,
			err,
		)
	}
}

func TestRetrievePubkey_ActionFailed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	// do not submit the keep public key intentionally to cause
	// the action error

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 3
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}
}

func TestRetrievePubkey_ContextCancelled_WithoutWorkingMonitoring(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	// cancel the context before any start event occurs
	cancelCtx()

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 0
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls != actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}
}

func TestRetrievePubkey_ContextCancelled_WithWorkingMonitoring(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	// wait a while before cancelling the context because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// cancel the context once the start event is handled and
	// the monitoring process is running
	cancelCtx()

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 0
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls !=
		actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}
}

func TestRetrievePubkey_OperatorNotInSigningGroup(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorRetrievePubKey(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := local.RandomSigningGroup(3)

	tbtcChain.CreateDeposit(depositAddress, signers)

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedRetrieveSignerPubkeyCalls := 0
	actualRetrieveSignerPubkeyCalls := tbtcChain.Logger().
		RetrieveSignerPubkeyCalls()
	if expectedRetrieveSignerPubkeyCalls !=
		actualRetrieveSignerPubkeyCalls {
		t.Errorf(
			"unexpected number of RetrieveSignerPubkey calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedRetrieveSignerPubkeyCalls,
			actualRetrieveSignerPubkeyCalls,
		)
	}
}

func TestProvideRedemptionSignature_TimeoutElapsed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 1
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}

	depositSignature, err := tbtcChain.DepositRedemptionSignature(
		depositAddress,
	)
	if err != nil {
		t.Errorf(
			"unexpected error while fetching deposit signature: [%v]",
			err,
		)
	}

	if !areChainSignaturesEqual(keepSignature, depositSignature) {
		t.Errorf(
			"unexpected signature\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			keepSignature,
			depositSignature,
		)
	}
}

func TestProvideRedemptionSignature_StopEventOccurred_DepositGotRedemptionSignature(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the stop event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// invoke the action which will trigger the stop event in result
	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 1
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}

	depositSignature, err := tbtcChain.DepositRedemptionSignature(
		depositAddress,
	)
	if err != nil {
		t.Errorf(
			"unexpected error while fetching deposit signature: [%v]",
			err,
		)
	}

	if !areChainSignaturesEqual(keepSignature, depositSignature) {
		t.Errorf(
			"unexpected signature\n"+
				"expected: [%+v]\n"+
				"actual:   [%+v]",
			keepSignature,
			depositSignature,
		)
	}
}

func TestProvideRedemptionSignature_StopEventOccurred_DepositRedeemed(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the stop event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// invoke the action which will trigger the stop event in result
	err = tbtcChain.ProvideRedemptionProof(
		depositAddress,
		[4]uint8{},
		nil,
		nil,
		[4]uint8{},
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}

	depositProof, err := tbtcChain.DepositRedemptionProof(depositAddress)
	if err != nil {
		t.Errorf("unexpected error while fetching deposit proof: [%v]", err)
	}

	if depositProof == nil {
		t.Errorf("deposit proof should be provided")
	}
}

func TestProvideRedemptionSignature_KeepClosedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep closed event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = closeKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}

	_, err = tbtcChain.DepositRedemptionSignature(depositAddress)

	expectedError := fmt.Errorf(
		"no redemption signature for deposit [%v]",
		depositAddress,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedError,
			err,
		)
	}
}

func TestProvideRedemptionSignature_KeepTerminatedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep terminated event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = terminateKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}

	_, err = tbtcChain.DepositRedemptionSignature(depositAddress)

	expectedError := fmt.Errorf(
		"no redemption signature for deposit [%v]",
		depositAddress,
	)
	if !reflect.DeepEqual(expectedError, err) {
		t.Errorf(
			"unexpected error\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedError,
			err,
		)
	}
}

func TestProvideRedemptionSignature_ActionFailed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// simulate a situation when `ProvideRedemptionSignature` fails on-chain
	tbtcChain.SetAlwaysFailingTransactions("ProvideRedemptionSignature")

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 3
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}
}

func TestProvideRedemptionSignature_ContextCancelled_WithoutWorkingMonitoring(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	// cancel the context before any start event occurs
	cancelCtx()

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}
}

func TestProvideRedemptionSignature_ContextCancelled_WithWorkingMonitoring(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before cancelling the context because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// cancel the context once the start event is handled and
	// the monitoring process is running
	cancelCtx()

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}
}

func TestProvideRedemptionSignature_OperatorNotInSigningGroup(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionSignature(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := local.RandomSigningGroup(3)

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	_, err = submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedProvideRedemptionSignatureCalls := 0
	actualProvideRedemptionSignatureCalls := tbtcChain.Logger().
		ProvideRedemptionSignatureCalls()
	if expectedProvideRedemptionSignatureCalls !=
		actualProvideRedemptionSignatureCalls {
		t.Errorf(
			"unexpected number of ProvideRedemptionSignature calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedProvideRedemptionSignatureCalls,
			actualProvideRedemptionSignatureCalls,
		)
	}
}

func TestProvideRedemptionProof_TimeoutElapsed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	initialDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 1
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}

	expectedDepositRedemptionFee := new(big.Int).Mul(
		big.NewInt(2),
		initialDepositRedemptionFee,
	)

	actualDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	if expectedDepositRedemptionFee.Cmp(actualDepositRedemptionFee) != 0 {
		t.Errorf(
			"unexpected redemption fee value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedDepositRedemptionFee.Text(10),
			actualDepositRedemptionFee.Text(10),
		)
	}
}

func TestProvideRedemptionProof_StopEventOccurred_DepositRedemptionRequested(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	initialDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the stop event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// invoke the action which will trigger the stop event in result
	err = tbtcChain.IncreaseRedemptionFee(
		depositAddress,
		toLittleEndianBytes(big.NewInt(990)),
		toLittleEndianBytes(big.NewInt(980)),
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	// Expect exactly one call of `IncreaseRedemptionFee` coming from the
	// explicit invocation placed above. The monitoring routine is not expected
	// to do any calls.
	expectedIncreaseRedemptionFeeCalls := 1
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}

	expectedDepositRedemptionFee := new(big.Int).Mul(
		big.NewInt(2),
		initialDepositRedemptionFee,
	)

	actualDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	if expectedDepositRedemptionFee.Cmp(actualDepositRedemptionFee) != 0 {
		t.Errorf(
			"unexpected redemption fee value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedDepositRedemptionFee.Text(10),
			actualDepositRedemptionFee.Text(10),
		)
	}
}

func TestProvideRedemptionProof_StopEventOccurred_DepositRedeemed(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the stop event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// invoke the action which will trigger the stop event in result
	err = tbtcChain.ProvideRedemptionProof(
		depositAddress,
		[4]uint8{},
		nil,
		nil,
		[4]uint8{},
		nil,
		nil,
		nil,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}

	depositProof, err := tbtcChain.DepositRedemptionProof(depositAddress)
	if err != nil {
		t.Errorf("unexpected error while fetching deposit proof: [%v]", err)
	}

	if depositProof == nil {
		t.Errorf("deposit proof should be provided")
	}
}

func TestProvideRedemptionProof_KeepClosedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	initialDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep closed event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = closeKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}

	actualDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	if initialDepositRedemptionFee.Cmp(actualDepositRedemptionFee) != 0 {
		t.Errorf(
			"unexpected redemption fee value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			initialDepositRedemptionFee.Text(10),
			actualDepositRedemptionFee.Text(10),
		)
	}
}

func TestProvideRedemptionProof_KeepTerminatedEventOccurred(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	initialDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before triggering the keep terminated event because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	err = terminateKeep(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}

	actualDepositRedemptionFee, err := tbtcChain.DepositRedemptionFee(
		depositAddress,
	)
	if err != nil {
		t.Fatal(err)
	}

	if initialDepositRedemptionFee.Cmp(actualDepositRedemptionFee) != 0 {
		t.Errorf(
			"unexpected redemption fee value\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			initialDepositRedemptionFee.Text(10),
			actualDepositRedemptionFee.Text(10),
		)
	}
}

func TestProvideRedemptionProof_ActionFailed(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// simulate a situation when `IncreaseRedemptionFee` fails on-chain
	tbtcChain.SetAlwaysFailingTransactions("IncreaseRedemptionFee")

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 3
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}
}

func TestProvideRedemptionProof_ContextCancelled_WithoutWorkingMonitoring(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	// cancel the context before any start event occurs
	cancelCtx()

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}
}

func TestProvideRedemptionProof_ContextCancelled_WithWorkingMonitoring(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := append(local.RandomSigningGroup(2), tbtcChain.Address())

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a while before cancelling the context because the
	// extension must have time to handle the start event
	time.Sleep(100 * time.Millisecond)

	// cancel the context once the start event is handled and
	// the monitoring process is running
	cancelCtx()

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}
}

func TestProvideRedemptionProof_OperatorNotInSigningGroup(
	t *testing.T,
) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	tbtcChain := local.NewTBTCLocalChain(ctx)
	tbtc := newTBTC(tbtcChain)

	err := tbtc.monitorProvideRedemptionProof(
		ctx,
		constantBackoff,
		timeout,
	)
	if err != nil {
		t.Fatal(err)
	}

	signers := local.RandomSigningGroup(3)

	tbtcChain.CreateDeposit(depositAddress, signers)

	_, err = submitKeepPublicKey(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.RedeemDeposit(depositAddress)
	if err != nil {
		t.Fatal(err)
	}

	keepSignature, err := submitKeepSignature(depositAddress, tbtcChain)
	if err != nil {
		t.Fatal(err)
	}

	err = tbtcChain.ProvideRedemptionSignature(
		depositAddress,
		keepSignature.V,
		keepSignature.R,
		keepSignature.S,
	)
	if err != nil {
		t.Fatal(err)
	}

	// wait a bit longer than the monitoring timeout
	// to make sure the potential transaction completes
	time.Sleep(2 * timeout)

	expectedIncreaseRedemptionFeeCalls := 0
	actualIncreaseRedemptionFeeCalls := tbtcChain.Logger().
		IncreaseRedemptionFeeCalls()
	if expectedIncreaseRedemptionFeeCalls != actualIncreaseRedemptionFeeCalls {
		t.Errorf(
			"unexpected number of IncreaseRedemptionFee calls\n"+
				"expected: [%v]\n"+
				"actual:   [%v]",
			expectedIncreaseRedemptionFeeCalls,
			actualIncreaseRedemptionFeeCalls,
		)
	}
}

func submitKeepPublicKey(
	depositAddress string,
	tbtcChain *local.TBTCLocalChain,
) ([64]byte, error) {
	keepAddress, err := tbtcChain.KeepAddress(depositAddress)
	if err != nil {
		return [64]byte{}, err
	}

	var keepPubkey [64]byte
	rand.Read(keepPubkey[:])

	err = tbtcChain.SubmitKeepPublicKey(
		common.HexToAddress(keepAddress),
		keepPubkey,
	)
	if err != nil {
		return [64]byte{}, err
	}

	return keepPubkey, nil
}

func submitKeepSignature(
	depositAddress string,
	tbtcChain *local.TBTCLocalChain,
) (*local.Signature, error) {
	keepAddress, err := tbtcChain.KeepAddress(depositAddress)
	if err != nil {
		return nil, err
	}

	signature := &ecdsa.Signature{
		R:          new(big.Int).SetUint64(rand.Uint64()),
		S:          new(big.Int).SetUint64(rand.Uint64()),
		RecoveryID: rand.Intn(4),
	}

	err = tbtcChain.SubmitSignature(
		common.HexToAddress(keepAddress),
		signature,
	)
	if err != nil {
		return nil, err
	}

	return toChainSignature(signature)
}

func toChainSignature(signature *ecdsa.Signature) (*local.Signature, error) {
	v := uint8(27 + signature.RecoveryID)

	r, err := byteutils.BytesTo32Byte(signature.R.Bytes())
	if err != nil {
		return nil, err
	}

	s, err := byteutils.BytesTo32Byte(signature.S.Bytes())
	if err != nil {
		return nil, err
	}

	return &local.Signature{
		V: v,
		R: r,
		S: s,
	}, nil
}

func areChainSignaturesEqual(signature1, signature2 *local.Signature) bool {
	if signature1.V != signature2.V {
		return false
	}

	if !bytes.Equal(signature1.R[:], signature2.R[:]) {
		return false
	}

	if !bytes.Equal(signature1.S[:], signature2.S[:]) {
		return false
	}

	return true
}

func closeKeep(
	depositAddress string,
	tbtcChain *local.TBTCLocalChain,
) error {
	keepAddress, err := tbtcChain.KeepAddress(depositAddress)
	if err != nil {
		return err
	}

	err = tbtcChain.CloseKeep(common.HexToAddress(keepAddress))
	if err != nil {
		return err
	}

	return nil
}

func terminateKeep(
	depositAddress string,
	tbtcChain *local.TBTCLocalChain,
) error {
	keepAddress, err := tbtcChain.KeepAddress(depositAddress)
	if err != nil {
		return err
	}

	err = tbtcChain.TerminateKeep(common.HexToAddress(keepAddress))
	if err != nil {
		return err
	}

	return nil
}

func constantBackoff(_ int) time.Duration {
	return time.Millisecond
}
