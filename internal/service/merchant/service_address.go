package merchant

import (
	"context"

	"github.com/google/uuid"
)

// GetMerchantAddressByUUID returns a saved merchant address by UUID.
// Deprecated: merchant_addresses table is legacy (non-custodial architecture).
// Withdrawals are performed directly by merchants on their smart contracts.
func (s *Service) GetMerchantAddressByUUID(ctx context.Context, merchantID int64, id uuid.UUID) (*Address, error) {
	return nil, ErrAddressNotFound
}

// GetMerchantAddressByID returns a saved merchant address by ID.
// Deprecated: merchant_addresses table is legacy (non-custodial architecture).
func (s *Service) GetMerchantAddressByID(ctx context.Context, merchantID, id int64) (*Address, error) {
	return nil, ErrAddressNotFound
}
