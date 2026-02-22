package blockchain

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	"github.com/antihax/optional"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	kms "github.com/cryptolink/cryptolink/internal/kms/wallet"
	"github.com/cryptolink/cryptolink/internal/money"
	"github.com/cryptolink/cryptolink/internal/provider/tatum"
	client "github.com/oxygenpay/tatum-sdk/tatum"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Broadcaster interface {
	BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, hex string, isTest bool) (string, error)
	GetTransactionReceipt(ctx context.Context, blockchain money.Blockchain, transactionID string, isTest bool) (*TransactionReceipt, error)
}

func (s *Service) BroadcastTransaction(ctx context.Context, blockchain money.Blockchain, rawTX string, isTest bool) (string, error) {
	api := s.providers.Tatum.Main()
	if isTest {
		api = s.providers.Tatum.Test()
	}

	var (
		txHash client.TransactionHash
		err    error
	)

	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		opts := &client.EthereumApiEthBroadcastOpts{}
		if isTest {
			opts.XTestnetType = optional.NewString(tatum.EthTestnet)
		}

		txHash, _, err = api.EthereumApi.EthBroadcast(ctx, client.BroadcastKms{TxData: rawTX}, opts)
	case kms.MATIC:
		txHash, _, err = api.PolygonApi.PolygonBroadcast(ctx, client.BroadcastKms{TxData: rawTX})
	case kms.BSC:
		txHash, _, err = api.BNBSmartChainApi.BscBroadcast(ctx, client.BroadcastKms{TxData: rawTX})
	case kms.TRON:
		hashID, errTron := s.providers.Trongrid.BroadcastTransaction(ctx, []byte(rawTX), isTest)
		if errTron != nil {
			err = errTron
		} else {
			txHash.TxId = hashID
		}
	case kms.ARBITRUM:
		rpc, errRPC := s.providers.Tatum.ArbitrumRPC(ctx, isTest)
		if errRPC != nil {
			return "", errors.Wrap(errRPC, "unable to get Arbitrum RPC")
		}
		defer rpc.Close()

		hashID, errBroadcast := s.broadcastRawTransaction(ctx, rpc, rawTX)
		if errBroadcast != nil {
			err = errBroadcast
		} else {
			txHash.TxId = hashID
		}
	case kms.AVAX:
		rpc, errRPC := s.providers.Tatum.AvalancheRPC(ctx, isTest)
		if errRPC != nil {
			return "", errors.Wrap(errRPC, "unable to get Avalanche RPC")
		}
		defer rpc.Close()

		hashID, errBroadcast := s.broadcastRawTransaction(ctx, rpc, rawTX)
		if errBroadcast != nil {
			err = errBroadcast
		} else {
			txHash.TxId = hashID
		}
	case kms.SOL:
		// Decode the raw transaction (base64 encoded signed transaction)
		hashID, errSolana := s.providers.Solana.SendTransaction(ctx, []byte(rawTX), isTest)
		if errSolana != nil {
			return "", errors.Wrap(errSolana, "unable to broadcast Solana transaction")
		}
		return hashID, nil
	case kms.XMR:
		// Monero uses a different approach - the transaction is created and broadcast via wallet-RPC
		// The rawTX here is actually the transfer params encoded as JSON
		return "", fmt.Errorf("Monero broadcasting is handled through wallet service, not via raw transaction")
	default:
		return "", fmt.Errorf("broadcast for %q is not implemented yet", blockchain)
	}

	if err != nil {
		errSwagger, ok := err.(client.GenericSwaggerError)
		if !ok {
			return "", errors.Wrap(err, "unknown swagger error")
		}

		s.logger.Error().Err(errSwagger).
			Str("raw_tx", rawTX).
			Str("response", string(errSwagger.Body())).
			Bool("is_test", isTest).
			Msg("unable to broadcast transaction")

		return "", parseBroadcastError(blockchain, errSwagger.Body())
	}

	return txHash.TxId, nil
}

type TransactionReceipt struct {
	Blockchain    money.Blockchain
	IsTest        bool
	Sender        string
	Recipient     string
	Hash          string
	Nonce         uint64
	NetworkFee    money.Money
	Success       bool
	Confirmations int64
	IsConfirmed   bool
}

func (s *Service) GetTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	receipt, err := s.getTransactionReceipt(ctx, blockchain, transactionID, isTest)

	if err != nil {
		errSwagger, ok := err.(client.GenericSwaggerError)
		if ok {
			err = errors.Errorf(string(errSwagger.Body()))
		}

		s.logger.Error().Err(err).Msg("unable to get transaction receipt")
	}

	return receipt, err
}

func (s *Service) getTransactionReceipt(
	ctx context.Context,
	blockchain money.Blockchain,
	transactionID string,
	isTest bool,
) (*TransactionReceipt, error) {
	const (
		ethConfirmations      = 12
		maticConfirmations    = 30
		bscConfirmations      = 15
		arbitrumConfirmations = 20
		avaxConfirmations     = 20
		solanaConfirmations   = 32
		moneroConfirmations   = 10
	)

	nativeCoin, err := s.GetNativeCoin(blockchain)
	if err != nil {
		return nil, errors.Wrapf(err, "native coin for %q is not found", blockchain)
	}

	switch kms.Blockchain(blockchain) {
	case kms.ETH:
		rpc, err := s.providers.Tatum.EthereumRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, ethConfirmations, isTest)
	case kms.MATIC:
		rpc, err := s.providers.Tatum.MaticRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, maticConfirmations, isTest)
	case kms.BSC:
		rpc, err := s.providers.Tatum.BinanceSmartChainRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, bscConfirmations, isTest)
	case kms.ARBITRUM:
		rpc, err := s.providers.Tatum.ArbitrumRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, arbitrumConfirmations, isTest)
	case kms.AVAX:
		rpc, err := s.providers.Tatum.AvalancheRPC(ctx, isTest)
		if err != nil {
			return nil, err
		}
		defer rpc.Close()

		return s.getEthReceipt(ctx, rpc, nativeCoin, transactionID, avaxConfirmations, isTest)
	case kms.TRON:
		receipt, err := s.providers.Trongrid.GetTransactionReceipt(ctx, transactionID, isTest)
		if err != nil {
			return nil, errors.Wrap(err, "unable to get tron transaction receipt")
		}

		networkFee, err := nativeCoin.MakeAmount(strconv.Itoa(int(receipt.Fee)))
		if err != nil {
			return nil, errors.Wrap(err, "unable to calculate network fee")
		}

		return &TransactionReceipt{
			Blockchain:    blockchain,
			IsTest:        isTest,
			Sender:        receipt.Sender,
			Recipient:     receipt.Recipient,
			Hash:          transactionID,
			NetworkFee:    networkFee,
			Success:       receipt.Success,
			Confirmations: receipt.Confirmations,
			IsConfirmed:   receipt.IsConfirmed,
		}, nil
	case kms.SOL:
		return s.getSolanaReceipt(ctx, nativeCoin, transactionID, solanaConfirmations, isTest)
	case kms.XMR:
		return s.getMoneroReceipt(ctx, nativeCoin, transactionID, moneroConfirmations, isTest)
	}

	return nil, kms.ErrUnknownBlockchain
}

func (s *Service) getEthReceipt(
	ctx context.Context,
	rpc *ethclient.Client,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	hash := common.HexToHash(txID)

	var (
		tx          *types.Transaction
		receipt     *types.Receipt
		latestBlock int64
		mu          sync.Mutex
		group       errgroup.Group
	)

	group.Go(func() error {
		txByHash, _, err := rpc.TransactionByHash(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		tx = txByHash
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		r, err := rpc.TransactionReceipt(ctx, hash)
		if err != nil {
			return err
		}

		mu.Lock()
		receipt = r
		mu.Unlock()

		return nil
	})

	group.Go(func() error {
		num, err := rpc.BlockNumber(ctx)
		if err != nil {
			return err
		}

		mu.Lock()
		latestBlock = int64(num)
		mu.Unlock()

		return nil
	})

	if err := group.Wait(); err != nil {
		return nil, err
	}

	gasPrice, err := nativeCoin.MakeAmountFromBigInt(receipt.EffectiveGasPrice)
	if err != nil {
		return nil, errors.Wrap(err, "unable to construct network fee")
	}

	networkFee, err := gasPrice.MultiplyInt64(int64(receipt.GasUsed))
	if err != nil {
		return nil, errors.Wrap(err, "unable to calculate network fee")
	}

	sender, err := types.Sender(types.LatestSignerForChainID(tx.ChainId()), tx)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get sender")
	}

	confirmations := latestBlock - receipt.BlockNumber.Int64()

	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        sender.String(),
		Recipient:     tx.To().String(),
		Hash:          txID,
		Nonce:         tx.Nonce(),
		NetworkFee:    networkFee,
		Success:       receipt.Status == 1,
		Confirmations: confirmations,
		IsConfirmed:   confirmations >= requiredConfirmations,
	}, nil
}

func parseBroadcastError(_ money.Blockchain, body []byte) error {
	// Sample response:
	//{
	//	"statusCode": 403,
	//	"errorCode": "eth.broadcast.failed",
	//	"message": "Unable to broadcast transaction.",
	//	"cause": "insufficient funds for gas * price + value [-32000]"
	//}
	type tatumErrObj struct {
		Message string `json:"message"`
		Cause   string `json:"cause"`
	}

	msg := &tatumErrObj{}
	_ = json.Unmarshal(body, msg)

	switch {
	case strings.Contains(msg.Message, "insufficient funds"):
		return ErrInsufficientFunds
	case strings.Contains(msg.Cause, "insufficient funds"):
		return ErrInsufficientFunds
	default:
		return errors.Wrap(ErrInvalidTransaction, msg.Message)
	}
}

// broadcastRawTransaction broadcasts a raw signed transaction to an EVM-compatible blockchain via RPC
func (s *Service) broadcastRawTransaction(ctx context.Context, rpc *ethclient.Client, rawTX string) (string, error) {
	// Decode the hex-encoded raw transaction
	tx := new(types.Transaction)
	txBytes := common.FromHex(rawTX)
	if err := tx.UnmarshalBinary(txBytes); err != nil {
		return "", errors.Wrap(err, "unable to decode raw transaction")
	}

	// Broadcast the transaction
	if err := rpc.SendTransaction(ctx, tx); err != nil {
		return "", errors.Wrap(err, "unable to send transaction")
	}

	return tx.Hash().Hex(), nil
}

// getSolanaReceipt retrieves transaction receipt from Solana blockchain
func (s *Service) getSolanaReceipt(
	ctx context.Context,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	// Confirm the transaction and get details
	confirmed, err := s.providers.Solana.ConfirmTransaction(ctx, txID, isTest, 30)
	if err != nil {
		return nil, errors.Wrap(err, "unable to confirm Solana transaction")
	}

	// For now, we'll return a basic receipt
	// TODO: Implement full transaction parsing to get sender, recipient, and fee details
	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        "", // Would need to parse transaction data
		Recipient:     "", // Would need to parse transaction data
		Hash:          txID,
		NetworkFee:    nativeCoin.MakeAmountMust("0.000005"), // Typical Solana fee is ~0.000005 SOL
		Success:       confirmed,
		Confirmations: requiredConfirmations, // Solana finality is very fast
		IsConfirmed:   confirmed,
	}, nil
}

// getMoneroReceipt retrieves transaction receipt from Monero blockchain
func (s *Service) getMoneroReceipt(
	ctx context.Context,
	nativeCoin money.CryptoCurrency,
	txID string,
	requiredConfirmations int64,
	isTest bool,
) (*TransactionReceipt, error) {
	// Monero transactions are managed through wallet-RPC
	// For now, return a placeholder receipt
	// TODO: Implement using Monero wallet-RPC GetTransfers and check transaction status
	return &TransactionReceipt{
		Blockchain:    nativeCoin.Blockchain,
		IsTest:        isTest,
		Sender:        "", // Monero privacy - sender not publicly visible
		Recipient:     "", // Recipient address from wallet
		Hash:          txID,
		NetworkFee:    nativeCoin.MakeAmountMust("0"), // Would need to query from wallet-RPC
		Success:       true,
		Confirmations: 0, // Would need to query from wallet-RPC
		IsConfirmed:   false,
	}, nil
}
