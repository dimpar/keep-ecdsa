package recovery

import (
	"bytes"
	cecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/keep-network/keep-ecdsa/pkg/ecdsa"
)

func TestPublicKeyToP2WPKHScriptCode(t *testing.T) {
	curve := elliptic.P224()
	privateKey := cecdsa.PrivateKey{
		PublicKey: cecdsa.PublicKey{
			Curve: curve,
			X:     bigIntFromString(t, "26941450295542185119886680310683442762751278660470797072401180634537"),
			Y:     bigIntFromString(t, "25316128528826716218547984440391195807075864123777900043328271050996"),
		},
		D: bigIntFromString(t, "18109202854347435965591271560665343234645589757372242840882235502272"),
	}
	scriptCodeBytes, err := PublicKeyToP2WPKHScriptCode(&privateKey.PublicKey, &chaincfg.TestNet3Params)

	if err != nil {
		t.Error(err)
	}

	if len(scriptCodeBytes) != 26 {
		t.Errorf("The script code must be exactly 26 bytes long. Instead, it was %v", len(scriptCodeBytes))
	}
}

func TestConstructUnsignedTransaction(t *testing.T) {
	recipientAddresses := []string{
		"bcrt1q5sz7jly79m76a5e8py6kv402q07p725vm4s0zl",
		"bcrt1qlxt5a04pefwkl90mna2sn79nu7asq3excx60h0",
		"bcrt1qjhpgmmhaxfwj6t7zf3dvs2fhdhx02g8qn3xwsf",
	}

	previousOutputValue := int64(100000000)

	messageTransaction, err := ConstructUnsignedTransaction(
		"0b99dea9655f219991001e9296cfe2103dd918a21ef477a14121d1a0ba9491f1",
		uint32(0),
		previousOutputValue,
		int64(700),
		recipientAddresses,
		&chaincfg.TestNet3Params,
	)
	if err != nil {
		t.Error(err)
	}

	serializedMessageTransactionBytes, _ := json.Marshal(messageTransaction)
	serializedMessageTransaction := string(serializedMessageTransactionBytes)
	expectedSerializedMessageTransaction := "{\"Version\":1,\"TxIn\":[{\"PreviousOutPoint\":{\"Hash\":[241,145,148,186,160,209,33,65,161,119,244,30,162,24,217,61,16,226,207,150,146,30,0,145,153,33,95,101,169,222,153,11],\"Index\":0},\"SignatureScript\":null,\"Witness\":[\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=\",\"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA\"],\"Sequence\":0}],\"TxOut\":[{\"Value\":33293200,\"PkScript\":\"ABSkBel8ni79rtMnCTVmVeoD/B8qjA==\"},{\"Value\":33293200,\"PkScript\":\"ABT5l06+ocpdb5X7n1UJ+LPnuwBHJg==\"},{\"Value\":33293200,\"PkScript\":\"ABSVwo3u/TJdLS/CTFrIKTdtzPUg4A==\"}],\"LockTime\":0}"
	if expectedSerializedMessageTransaction != serializedMessageTransaction {
		t.Errorf(
			"expectedSerializedMessageTransaction did not match the expectation\nexpected: %s\nactual:   %s",
			expectedSerializedMessageTransaction,
			serializedMessageTransaction,
		)
	}
}

func Test_BuildSignedTransactionHexString(t *testing.T) {
	unsignedTransaction := wire.NewMsgTx(wire.TxVersion)
	previousOutputTransactionHash, err := chainhash.NewHashFromStr(
		"0b99dea9655f219991001e9296cfe2103dd918a21ef477a14121d1a0ba9491f1",
	)
	if err != nil {
		t.Errorf("Something went wrong generating the previousOutputTransactionHash: %v", err)
	}
	unsignedTransaction.AddTxIn(wire.NewTxIn(
		wire.NewOutPoint(previousOutputTransactionHash, 0),
		nil,
		nil,
	))

	for _, txValue := range []int{100, 100, 100} {
		unsignedTransaction.AddTxOut(wire.NewTxOut(
			int64(txValue),
			nil,
		))
	}
	curve := elliptic.P224()
	privateKey := cecdsa.PrivateKey{
		PublicKey: cecdsa.PublicKey{
			Curve: curve,
			X:     bigIntFromString(t, "828612351041249926199933036276541218289243364325366441967565889653"),
			Y:     bigIntFromString(t, "985040320797760939221216987624001720525496952574017416820319442840"),
		},
		D: bigIntFromString(t, "13944630350777130481687329643969358734978178969757056469087366016470"),
	}
	signedTransactionHex, err := BuildSignedTransactionHexString(
		unsignedTransaction,
		&ecdsa.Signature{
			R:          big.NewInt(int64(3)),
			S:          big.NewInt(int64(7)),
			RecoveryID: 1,
		},
		&privateKey.PublicKey,
	)
	if err != nil {
		t.Errorf("Something went wrong building the signed transaction string: %v", err)
	}

	signedTransactionBytes, _ := hex.DecodeString(signedTransactionHex)
	signedTransaction := wire.NewMsgTx(wire.TxVersion)
	signedTransaction.BtcDecode(bytes.NewReader(signedTransactionBytes), 1, wire.WitnessEncoding)
	serializedSignedTransactionBytes, _ := json.Marshal(signedTransaction)
	serializedSignedTransaction := string(serializedSignedTransactionBytes)
	expectedSerializedSignedTransaction := "{\"Version\":1,\"TxIn\":[{\"PreviousOutPoint\":{\"Hash\":[241,145,148,186,160,209,33,65,161,119,244,30,162,24,217,61,16,226,207,150,146,30,0,145,153,33,95,101,169,222,153,11],\"Index\":0},\"SignatureScript\":\"\",\"Witness\":[\"MAYCAQMCAQcB\",\"AgAAAAAH3j67ZA0rAhWQwJ1ec5WX0C2TkiTSJ6F0A2B1\"],\"Sequence\":4294967295}],\"TxOut\":[{\"Value\":100,\"PkScript\":\"\"},{\"Value\":100,\"PkScript\":\"\"},{\"Value\":100,\"PkScript\":\"\"}],\"LockTime\":0}"

	if expectedSerializedSignedTransaction != serializedSignedTransaction {
		t.Errorf(
			"the signed transaction does not match expectation\nexpected: %s\nactual:   %s",
			expectedSerializedSignedTransaction,
			serializedSignedTransaction,
		)
	}

	if len(signedTransaction.TxIn) != len(unsignedTransaction.TxIn) {
		t.Errorf(
			"the original and signed transactions must have the same number of input transactions\n"+
				"expected: %v\nactual:   %v",
			len(unsignedTransaction.TxIn),
			len(signedTransaction.TxIn),
		)
	}
}

func bigIntFromString(t *testing.T, s string) *big.Int {
	bigInt, ok := new(big.Int).SetString(s, 0)
	if !ok {
		t.Errorf("Something went wrong creating a big int from %s", s)
	}
	return bigInt
}
