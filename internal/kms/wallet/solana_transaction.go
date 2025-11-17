package wallet

import (
	"crypto/ed25519"
	"encoding/binary"
	"fmt"

	"github.com/mr-tron/base58"
	"github.com/pkg/errors"
)

// SolanaTransactionParams parameters for creating Solana transactions
type SolanaTransactionParams struct {
	Type AssetType

	FromAddress     string
	Recipient       string
	Amount          uint64 // Amount in lamports (1 SOL = 1,000,000,000 lamports)
	TokenMint       string // For SPL tokens, the mint address
	RecentBlockhash string // Required for transaction validity
}

// SolanaTransaction represents a Solana transaction
type SolanaTransaction struct {
	RawTransaction []byte
	Signature      string
	TxHash         string
}

func (p SolanaTransactionParams) validate() error {
	if !p.Type.Valid() {
		return errors.New("type is invalid")
	}

	if !validateSolanaAddress(p.Recipient) {
		return ErrInvalidAddress
	}

	if !validateSolanaAddress(p.FromAddress) {
		return ErrInvalidAddress
	}

	if p.Type == Token && !validateSolanaAddress(p.TokenMint) {
		return errors.New("invalid token mint address")
	}

	if p.Amount == 0 {
		return ErrInvalidAmount
	}

	if p.RecentBlockhash == "" {
		return errors.New("recent blockhash required")
	}

	return nil
}

// CreateSolanaTransaction creates and signs a Solana transaction
// For native SOL transfers or SPL token transfers
func (p *SolanaProvider) CreateSolanaTransaction(params SolanaTransactionParams, privateKeyBase58 string) (*SolanaTransaction, error) {
	if err := params.validate(); err != nil {
		return nil, errors.Wrap(err, "invalid transaction parameters")
	}

	// Decode private key
	privKeyBytes, err := base58.Decode(privateKeyBase58)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode private key")
	}

	if len(privKeyBytes) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key size")
	}

	privateKey := ed25519.PrivateKey(privKeyBytes)

	// Decode addresses
	fromPubKey, err := base58.Decode(params.FromAddress)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode from address")
	}

	toPubKey, err := base58.Decode(params.Recipient)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode recipient address")
	}

	recentBlockhash, err := base58.Decode(params.RecentBlockhash)
	if err != nil {
		return nil, errors.Wrap(err, "failed to decode recent blockhash")
	}

	var txBytes []byte
	var txType string

	if params.Type == Coin {
		// Native SOL transfer
		txBytes, err = p.createNativeTransfer(fromPubKey, toPubKey, params.Amount, recentBlockhash)
		txType = "native_sol_transfer"
	} else {
		// SPL token transfer
		tokenMint, decodeErr := base58.Decode(params.TokenMint)
		if decodeErr != nil {
			return nil, errors.Wrap(decodeErr, "failed to decode token mint")
		}
		txBytes, err = p.createSPLTokenTransfer(fromPubKey, toPubKey, tokenMint, params.Amount, recentBlockhash)
		txType = "spl_token_transfer"
	}

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create %s", txType)
	}

	// Sign transaction
	signature := ed25519.Sign(privateKey, txBytes)

	// Combine signature + transaction
	signedTx := append([]byte{1}, signature...) // 1 signature
	signedTx = append(signedTx, txBytes...)

	return &SolanaTransaction{
		RawTransaction: signedTx,
		Signature:      base58.Encode(signature),
		TxHash:         base58.Encode(signature), // In Solana, signature IS the tx hash
	}, nil
}

// createNativeTransfer creates a native SOL transfer instruction
// This is a simplified implementation - production should use solana-go SDK
func (p *SolanaProvider) createNativeTransfer(from, to []byte, lamports uint64, recentBlockhash []byte) ([]byte, error) {
	// Solana transaction structure (simplified):
	// - Header (3 bytes)
	// - Account addresses (compact array)
	// - Recent blockhash (32 bytes)
	// - Instructions (compact array)

	// System Program ID for transfers
	systemProgram := make([]byte, 32) // All zeros = System Program

	// Build transaction message
	message := []byte{}

	// Header: 1 required signature, 0 readonly signed, 1 readonly unsigned
	header := []byte{1, 0, 1}
	message = append(message, header...)

	// Account addresses (compact array of 3 accounts)
	message = append(message, 3) // 3 accounts
	message = append(message, from...)
	message = append(message, to...)
	message = append(message, systemProgram...)

	// Recent blockhash
	message = append(message, recentBlockhash...)

	// Instructions (compact array with 1 instruction)
	message = append(message, 1) // 1 instruction

	// Transfer instruction
	instruction := []byte{}
	instruction = append(instruction, 2)          // Program account index (System Program)
	instruction = append(instruction, 2)          // 2 accounts involved
	instruction = append(instruction, 0)          // From account index
	instruction = append(instruction, 1)          // To account index
	instruction = append(instruction, 12)         // Instruction data length
	instruction = append(instruction, 2, 0, 0, 0) // Transfer instruction type

	// Amount in lamports (8 bytes, little endian)
	amountBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(amountBytes, lamports)
	instruction = append(instruction, amountBytes...)

	message = append(message, instruction...)

	return message, nil
}

// createSPLTokenTransfer creates an SPL token transfer instruction
// This is a simplified implementation - production should use solana-go SDK
func (p *SolanaProvider) createSPLTokenTransfer(from, to, mint []byte, amount uint64, recentBlockhash []byte) ([]byte, error) {
	// SPL Token transfers are more complex and require:
	// 1. Finding the associated token accounts
	// 2. Creating transfer instruction with Token Program
	// 3. Possibly creating associated token account if it doesn't exist

	// NOTE: This is a placeholder. Real implementation needs:
	// - solana-go SDK for proper SPL token instruction encoding
	// - Associated Token Account (ATA) derivation
	// - Token Program invocation

	return nil, errors.New("SPL token transfers require solana-go SDK - install with: go get github.com/gagliardetto/solana-go")
}

// ValidateAddress performs proper Solana address validation
func ValidateSolanaAddressWithChecksum(address string) error {
	// Decode base58
	decoded, err := base58.Decode(address)
	if err != nil {
		return errors.Wrap(ErrInvalidAddress, "invalid base58 encoding")
	}

	// Solana public keys are exactly 32 bytes
	if len(decoded) != 32 {
		return errors.Wrapf(ErrInvalidAddress, "invalid length: %d (expected 32)", len(decoded))
	}

	// Additional validation: Ensure it's a valid ed25519 public key point
	// This is a basic check - real validation would verify the point is on the curve
	if len(decoded) != ed25519.PublicKeySize {
		return ErrInvalidAddress
	}

	return nil
}

// Helper function to convert SOL to lamports
func SOLToLamports(sol float64) uint64 {
	return uint64(sol * 1_000_000_000)
}

// Helper function to convert lamports to SOL
func LamportsToSOL(lamports uint64) float64 {
	return float64(lamports) / 1_000_000_000
}

// GetSolanaExplorerURL returns the block explorer URL for a transaction
func GetSolanaExplorerURL(signature string, isTestnet bool) string {
	if isTestnet {
		return fmt.Sprintf("https://explorer.solana.com/tx/%s?cluster=devnet", signature)
	}
	return fmt.Sprintf("https://explorer.solana.com/tx/%s", signature)
}
