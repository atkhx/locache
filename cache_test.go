package locache

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCache = Cache[string, string]

func requireValue(t *testing.T, cache *testCache, key, expectedValue string, expectedExists bool) {
	t.Helper()
	v, ok := cache.Get(key)
	require.Equal(t, expectedValue, v)
	require.Equal(t, expectedExists, ok)
}

func requireKeyExists(t *testing.T, cache *testCache, key, value string) {
	requireValue(t, cache, key, value, true)
}

func requireKeyNotExists(t *testing.T, cache *testCache, key string) {
	requireValue(t, cache, key, "", false)
}

func requireNoItem(t *testing.T, cache *testCache, key string) {
	t.Helper()
	item, ok := cache.itemsMap.Get(key)
	require.Nil(t, item)
	require.False(t, ok)
}

func requireItemExists(t *testing.T, cache *testCache, key string) {
	t.Helper()
	_, ok := cache.itemsMap.Get(key)
	require.True(t, ok)
}

func TestCache_Purge_Manually(t *testing.T) {
	cache := New[string, string](context.Background(), time.Nanosecond)
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// These items should be created with ttl 3 Seconds
	cache.itemsTTL = 3 * time.Second
	cache.Set("key3", "value3")
	cache.Set("key4", "value4")

	// Sleep a nanosecond to avoid flaky test on expiration key0, key1, key2.
	time.Sleep(time.Nanosecond)

	// After a nanosecond values should become expired.
	requireKeyNotExists(t, cache, "key0")
	requireKeyNotExists(t, cache, "key1")
	requireKeyNotExists(t, cache, "key2")

	// We don't schedule automatically purge, so items should be sill in the map.
	requireItemExists(t, cache, "key0")
	requireItemExists(t, cache, "key1")
	requireItemExists(t, cache, "key2")

	// Call purge manually will remove expired items from map.
	cache.Purge()

	requireNoItem(t, cache, "key0")
	requireNoItem(t, cache, "key1")
	requireNoItem(t, cache, "key2")

	requireKeyExists(t, cache, "key3", "value3")
	requireKeyExists(t, cache, "key4", "value4")
}

func TestCache_Get_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_Get_KeyExpired(t *testing.T) {
	cache := New[string, string](context.Background(), 0)
	cache.Set("key0", "value0")
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_GetOrRefresh_KeyNotExists(t *testing.T) {
	calls := atomic.Int32{}
	cache := New[string, string](context.Background(), time.Second)

	actual, err := cache.GetOrRefresh("key0", func() (string, error) {
		calls.Add(1)
		return "value0", nil
	})
	require.Equal(t, int32(1), calls.Load())
	require.NoError(t, err)
	require.Equal(t, "value0", actual)

	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_GetOrRefresh_KeyExistsAndValid(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")
	actual, err := cache.GetOrRefresh("key0", func() (string, error) {
		panic("should never be called")
	})
	require.NoError(t, err)
	require.Equal(t, "value0", actual)

	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_GetOrRefresh_KeyExistsAndNotValid(t *testing.T) {
	calls := atomic.Int32{}
	cache := New[string, string](context.Background(), 0)
	cache.Set("key0", "value0")
	// For testing purpose only
	cache.itemsTTL = time.Second

	actual, err := cache.GetOrRefresh("key0", func() (string, error) {
		calls.Add(1)
		return "updated", nil
	})
	require.Equal(t, int32(1), calls.Load())
	require.NoError(t, err)
	require.Equal(t, "updated", actual)

	requireKeyExists(t, cache, "key0", "updated")
}

func TestCache_GetOrRefresh_RefreshFailed(t *testing.T) {
	var originErr = fmt.Errorf("some error")

	calls := atomic.Int32{}
	cache := New[string, string](context.Background(), time.Second)
	actual, err := cache.GetOrRefresh("key0", func() (string, error) {
		calls.Add(1)
		return "", originErr
	})

	require.Equal(t, int32(1), calls.Load())
	require.ErrorIs(t, err, originErr)
	require.Empty(t, actual)

	requireNoItem(t, cache, "key0")
}

func TestCache_Set_InsertValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_Set_UpdateValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Set("key1", "updated")
	requireKeyExists(t, cache, "key0", "value0")
	requireKeyExists(t, cache, "key1", "updated")
	requireKeyExists(t, cache, "key2", "value2")
}

func TestCache_Update_KeyNotExists_SetNewValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.False(t, exists)
		return "value0", true, nil
	})

	require.NoError(t, actualErr)
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_Update_KeyNotExists_NotSetValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.False(t, exists)
		return "value0", false, nil
	})

	require.NoError(t, actualErr)
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_Update_KeyNotExists_GetValueFailed(t *testing.T) {
	var expectedErr = fmt.Errorf("some error")
	cache := New[string, string](context.Background(), time.Second)
	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.False(t, exists)
		return "", false, expectedErr
	})

	require.ErrorIs(t, actualErr, expectedErr)
	requireNoItem(t, cache, "key0")
}

func TestCache_Update_KeyExists_SetValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")

	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.True(t, exists)
		return "value1", true, nil
	})

	require.NoError(t, actualErr)
	requireKeyExists(t, cache, "key0", "value1")
}

func TestCache_Update_KeyExists_NotSetValue(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")

	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.True(t, exists)
		return "value1", false, nil
	})

	require.NoError(t, actualErr)
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_Update_KeyExists_GetValueFailed(t *testing.T) {
	var expectedErr = fmt.Errorf("some error")

	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")

	actualErr := cache.Update("key0", func(value string, exists bool) (string, bool, error) {
		require.True(t, exists)
		return "", false, expectedErr
	})

	require.ErrorIs(t, actualErr, expectedErr)
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_Delete_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Delete("key0")
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_Delete_KeyExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	cache.Delete("key1")
	requireKeyExists(t, cache, "key0", "value0")
	requireKeyNotExists(t, cache, "key1")
	requireKeyExists(t, cache, "key2", "value2")
}

func TestCache_DeleteExpired_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.DeleteExpired("key0")
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_DeleteExpired_KeyNotExpired(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second)
	cache.Set("key0", "value0")
	cache.DeleteExpired("key0")
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_DeleteExpired_KeyExpired(t *testing.T) {
	cache := New[string, string](context.Background(), 0)
	cache.Set("key0", "value0")
	cache.itemsTTL = time.Second

	cache.DeleteExpired("key0")
	requireKeyNotExists(t, cache, "key0")
}
