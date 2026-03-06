package wallet

import (
	"crypto/ed25519"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/mr-tron/base58"
)

// SolanaProvider generates wallets for Solana blockchain
type SolanaProvider struct {
	Blockchain   Blockchain
	CryptoReader io.Reader
}

func (p *SolanaProvider) Generate() *Wallet {
	// Generate ed25519 keypair for Solana
	publicKey, privateKey, err := ed25519.GenerateKey(p.CryptoReader)
	if err != nil {
		return &Wallet{}
	}

	// Solana addresses are base58-encoded public keys
	address := base58.Encode(publicKey)
	pubKeyEncoded := base58.Encode(publicKey)
	privKeyEncoded := base58.Encode(privateKey)

	return &Wallet{
		UUID:       uuid.New(),
		CreatedAt:  time.Now(),
		Blockchain: p.Blockchain,
		Address:    address,
		PublicKey:  pubKeyEncoded,
		PrivateKey: privKeyEncoded,
	}
}

func (p *SolanaProvider) GetBlockchain() Blockchain {
	return p.Blockchain
}

func (p *SolanaProvider) ValidateAddress(address string) bool {
	return validateSolanaAddress(address)
}
