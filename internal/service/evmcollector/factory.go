package evmcollector

import (
	"context"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// CollectorFactory represents a deployed clone factory + implementation pair for an EVM chain.
type CollectorFactory struct {
	ID                    int64
	Blockchain            string
	ImplementationAddress string
	FactoryAddress        string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

var ErrFactoryNotFound = errors.New("collector factory not found")

// GetFactoryByBlockchain retrieves the factory config for a specific blockchain.
func (s *Service) GetFactoryByBlockchain(ctx context.Context, blockchain string) (*CollectorFactory, error) {
	blockchain = strings.ToUpper(blockchain)

	f := &CollectorFactory{}
	err := s.db.QueryRow(ctx, `
		SELECT id, blockchain, implementation_address, factory_address, created_at, updated_at
		FROM collector_factories
		WHERE blockchain = $1
	`, blockchain).Scan(
		&f.ID, &f.Blockchain, &f.ImplementationAddress, &f.FactoryAddress,
		&f.CreatedAt, &f.UpdatedAt,
	)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return nil, ErrFactoryNotFound
		}
		return nil, errors.Wrap(err, "unable to get collector factory")
	}
	return f, nil
}

// ListFactories returns all collector factory configs.
func (s *Service) ListFactories(ctx context.Context) ([]*CollectorFactory, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, blockchain, implementation_address, factory_address, created_at, updated_at
		FROM collector_factories
		ORDER BY blockchain
	`)
	if err != nil {
		return nil, errors.Wrap(err, "unable to list collector factories")
	}
	defer rows.Close()

	var factories []*CollectorFactory
	for rows.Next() {
		f := &CollectorFactory{}
		if err := rows.Scan(
			&f.ID, &f.Blockchain, &f.ImplementationAddress, &f.FactoryAddress,
			&f.CreatedAt, &f.UpdatedAt,
		); err != nil {
			return nil, errors.Wrap(err, "unable to scan collector factory")
		}
		factories = append(factories, f)
	}
	return factories, nil
}

// UpsertFactory creates or updates a collector factory config for a blockchain.
func (s *Service) UpsertFactory(ctx context.Context, factory *CollectorFactory) (*CollectorFactory, error) {
	factory.Blockchain = strings.ToUpper(factory.Blockchain)
	// Note: do NOT lowercase addresses — TRON uses base58 which is case-sensitive

	now := time.Now().UTC().Truncate(time.Second)

	_, err := s.db.Exec(ctx, `
		INSERT INTO collector_factories
			(blockchain, implementation_address, factory_address, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		ON CONFLICT (blockchain) DO UPDATE SET
			implementation_address = EXCLUDED.implementation_address,
			factory_address        = EXCLUDED.factory_address,
			updated_at             = EXCLUDED.updated_at
	`, factory.Blockchain, factory.ImplementationAddress, factory.FactoryAddress, now)

	if err != nil {
		return nil, errors.Wrap(err, "unable to upsert collector factory")
	}

	return s.GetFactoryByBlockchain(ctx, factory.Blockchain)
}
