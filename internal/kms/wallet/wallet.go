// Package wallet is used inside KMS to provide features related to wallet generation & CRUD access.
package wallet

import (
	"time"

	"github.com/google/uuid"
	"github.com/oxygenpay/oxygen/internal/money"
	"github.com/pkg/errors"
)

type Blockchain string

const (
	BTC      Blockchain = "BTC"
	ETH      Blockchain = "ETH"
	TRON     Blockchain = "TRON"
	MATIC    Blockchain = "MATIC"
	BSC      Blockchain = "BSC"
	ARBITRUM Blockchain = "ARBITRUM"
	AVAX     Blockchain = "AVAX"
	SOL      Blockchain = "SOL"
	XMR      Blockchain = "XMR"
)

var blockchains = []Blockchain{BTC, ETH, TRON, MATIC, BSC, ARBITRUM, AVAX, SOL, XMR}

func ListBlockchains() []Blockchain {
	result := make([]Blockchain, len(blockchains))
	copy(result, blockchains)

	return result
}

type Wallet struct {
	UUID       uuid.UUID  `json:"uuid"`
	Address    string     `json:"address"`
	PublicKey  string     `json:"public_key"`
	PrivateKey string     `json:"private_key"`
	CreatedAt  time.Time  `json:"created_at"`
	DeletedAt  *time.Time `json:"deleted_at"`
	Blockchain Blockchain `json:"blockchain"`
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

func ValidateAddress(blockchain Blockchain, address string) error {
	var isValid bool
	switch blockchain {
	case BTC:
		isValid = validateBitcoinAddress(address)
	case ETH, MATIC, BSC, ARBITRUM, AVAX:
		isValid = validateEthereumAddress(address)
	case TRON:
		isValid = validateTronAddress(address)
	case SOL:
		isValid = validateSolanaAddress(address)
	case XMR:
		isValid = validateMoneroAddress(address)
	default:
		return errors.Wrapf(ErrUnknownBlockchain, "unknown blockchain %q", blockchain)
	}

	if !isValid {
		return ErrInvalidAddress
	}

	return nil
}

// validateSolanaAddress validates Solana addresses (base58, 32-44 characters)
func validateSolanaAddress(address string) bool {
	if len(address) < 32 || len(address) > 44 {
		return false
	}
	// Basic check - Solana addresses are base58 encoded
	for _, c := range address {
		if !((c >= '1' && c <= '9') || (c >= 'A' && c <= 'H') || (c >= 'J' && c <= 'N') ||
			(c >= 'P' && c <= 'Z') || (c >= 'a' && c <= 'k') || (c >= 'm' && c <= 'z')) {
			return false
		}
	}
	return true
}

// validateMoneroAddress validates Monero addresses (starts with 4 or 8, 95 characters)
func validateMoneroAddress(address string) bool {
	if len(address) != 95 {
		return false
	}
	// Standard Monero addresses start with '4'
	// Integrated addresses start with '8'
	if address[0] != '4' && address[0] != '8' {
		return false
	}
	return true
}
