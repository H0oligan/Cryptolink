// Package wallet defines Blockchain type constants used across the codebase.
// The KMS wallet generation and signing functionality has been removed.
// CryptoLink is non-custodial: merchants withdraw directly via MetaMask/TronLink.
package wallet

import (
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/pkg/errors"
)

var ErrUnknownBlockchain = errors.New("unknown blockchain")

type Blockchain string

const (
	BTC      Blockchain = "BTC"
	ETH      Blockchain = "ETH"
	TRON     Blockchain = "TRON"
	MATIC    Blockchain = "MATIC"
	BSC      Blockchain = "BSC"
	ARBITRUM Blockchain = "ARBITRUM"
	AVAX     Blockchain = "AVAX"
)

var blockchains = []Blockchain{BTC, ETH, TRON, MATIC, BSC, ARBITRUM, AVAX}

func ListBlockchains() []Blockchain {
	result := make([]Blockchain, len(blockchains))
	copy(result, blockchains)

	return result
}

func (b Blockchain) IsValid() bool {
	for _, bc := range blockchains {
		if b == bc {
			return true
		}
	}

	return false
}

func (b Blockchain) ToMoneyBlockchain() money.Blockchain {
	return money.Blockchain(b)
}

func (b Blockchain) String() string {
	return string(b)
}

func (b Blockchain) NotSpecified() bool {
	return b == ""
}

func (b Blockchain) IsSpecified() bool {
	return b != ""
}
