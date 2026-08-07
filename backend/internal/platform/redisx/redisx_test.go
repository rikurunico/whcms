package redisx_test

import (
	"context"
	"testing"

	"github.com/tsdlamongan/whcms/backend/internal/platform/redisx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestKey(t *testing.T) {
	assert.Equal(t, "whmcs:refresh:abc", redisx.Key("refresh", "abc"))
	assert.Equal(t, "whmcs:lock:cron:invoices", redisx.Key("lock", "cron:invoices"))
	assert.Equal(t, "whmcs:solo", redisx.Key("solo"))
}

func TestConnect(t *testing.T) {
	rdb, err := redisx.Connect(context.Background(), "localhost:6379", "", 15)
	if err != nil {
		t.Skipf("redis not available, skipping: %v", err)
	}
	defer rdb.Close()
	require.NoError(t, rdb.Ping(context.Background()).Err())
}

func TestConnectBadAddr(t *testing.T) {
	_, err := redisx.Connect(context.Background(), "127.0.0.1:1", "", 0)
	assert.Error(t, err)
}
