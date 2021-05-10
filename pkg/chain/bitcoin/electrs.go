package bitcoin

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/ipfs/go-log"
)

var logger = log.Logger("bitcoin")

type httpClient interface {
	Post(url string, contentType string, body io.Reader) (*http.Response, error)
	Get(url string) (*http.Response, error)
}

// ElectrsConnection exposes a native API for interacting with an electrs http API.
type ElectrsConnection struct {
	apiURL string
	client httpClient
}

// NewElectrsConnection is a constructor for ElectrsConnection.
func NewElectrsConnection(apiURL string) *ElectrsConnection {
	return &ElectrsConnection{
		apiURL: apiURL,
		client: http.DefaultClient,
	}
}

func (e *ElectrsConnection) setClient(client httpClient) {
	e.client = client
}

// Broadcast broadcasts a transaction the configured bitcoin network.
func (e ElectrsConnection) Broadcast(transaction string) error {
	if e.apiURL == "" {
		for i := 0; i < 5; i++ {
			logger.Warningf("Please broadcast Bitcoin transaction %s", transaction)
		}
		return nil
	}
	resp, err := e.client.Post(fmt.Sprintf("%s/tx", e.apiURL), "text/plain", strings.NewReader(transaction))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("something went wrong with broadcast: [%s], transaction: [%s]", resp.Status, transaction)
	}
	transactionIDBuffer := new(strings.Builder)
	bytesCopied, err := io.Copy(transactionIDBuffer, resp.Body)
	// if the status code was 200, but we were unable to read the body, log an
	// error but return successfully anyway.
	if err != nil {
		logger.Errorf("something went wrong reading the electrs response body: [%v]", err)
	}
	if bytesCopied == 0 {
		logger.Error("something went wrong reading the electrs response body: 0 bytes copied")
	}
	logger.Infof("successfully broadcast the bitcoin transaction: %s", transactionIDBuffer.String())
	return nil
}

// VbyteFee retrieves the 25-block estimate fee per vbyte on the bitcoin network.
func (e ElectrsConnection) VbyteFee() (int32, error) {
	if e.apiURL == "" {
		return 0, nil
	}
	resp, err := e.client.Get(fmt.Sprintf("%s/fee-estimates", e.apiURL))
	if err != nil {
		return 0, err
	}
	if resp.StatusCode != 200 {
		return 0, fmt.Errorf("something went wrong with broadcast: [%s]", resp.Status)
	}

	var fees map[string]float32
	err = json.NewDecoder(resp.Body).Decode(&fees)
	if err != nil {
		return 0, fmt.Errorf("something went wrong decoding the vbyte fees: [%v]", err)
	}
	fee, ok := fees["25"]
	if !ok {
		fee = 0
	}
	logger.Info("retrieved a vbyte fee of [%d]", fee)
	return int32(fee), nil
}

func (e ElectrsConnection) IsAddressUnused(btcAddress string) (bool, error) {
	if e.apiURL == "" {
		return true, nil
	}
	resp, err := e.client.Get(fmt.Sprintf("%s/address/%s/txs", e.apiURL, btcAddress))
	if err != nil {
		return false, err
	}
	if resp.StatusCode != 200 {
		return false, fmt.Errorf(
			"something went wrong trying to get information about address %s: [%s]",
			btcAddress,
			resp.Status,
		)
	}

	responses := []interface{}{}
	err = json.NewDecoder(resp.Body).Decode(&responses)
	if err != nil {
		return false, err
	}

	return len(responses) == 0, nil
}
