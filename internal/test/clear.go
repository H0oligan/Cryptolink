package test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type Clear struct {
	tc *IntegrationTest
}

func (c *Clear) Wallets(t *testing.T) {
	ctx := context.Background()

	_, err := c.tc.Database.connection.Exec(ctx, "delete from balances where id > 0;")
	require.NoError(t, err)

	c.tc.Providers.PriceFeedMock.Clear()
}

func (c *Clear) Table(t *testing.T, table string) {
	ctx := context.Background()

	_, err := c.tc.Database.connection.Exec(ctx, "truncate table "+table)
	require.NoError(t, err)
}
