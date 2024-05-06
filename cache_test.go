package locache

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type testCache = Cache[string, string]

func requireGetResult(t *testing.T, cache *testCache, key, expectedValue string, expectedExists bool) {
	t.Helper()
	v, ok := cache.Get(key)
	require.Equal(t, expectedValue, v)
	require.Equal(t, expectedExists, ok)
}

func requireKeyExists(t *testing.T, cache *testCache, key, value string) {
	requireGetResult(t, cache, key, value, true)
}

func requireKeyNotExists(t *testing.T, cache *testCache, key string) {
	requireGetResult(t, cache, key, "", false)
}

func TestCache_Get_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	requireKeyNotExists(t, cache, "key0")
}

func requireCacheItems(t *testing.T, cache *testCache, expected []string) {
	t.Helper()
	actual := make([]string, 0, len(expected))
	for element := cache.items.Front(); element != nil; element = element.Next() {
		actual = append(actual, element.Value.(*Item[string, string]).val)
	}
	require.Equal(t, expected, actual)
}

func TestCache_Get_KeyExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	requireKeyExists(t, cache, "key0", "value0")
	requireKeyExists(t, cache, "key1", "value1")
	requireKeyExists(t, cache, "key2", "value2")
}

func TestCache_Set_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	cache.Set("key0", "value0")
	requireKeyExists(t, cache, "key0", "value0")
}

func TestCache_Set_KeyExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	requireKeyExists(t, cache, "key0", "value0")
	requireKeyExists(t, cache, "key1", "value1")
	requireKeyExists(t, cache, "key2", "value2")

	cache.Set("key1", "updated1")
	requireKeyExists(t, cache, "key1", "updated1")
	requireCacheItems(t, cache, []string{"value0", "value2", "updated1"})

	cache.Set("key1", "updated11")
	requireKeyExists(t, cache, "key1", "updated11")
	requireCacheItems(t, cache, []string{"value0", "value2", "updated11"})

	cache.Set("key0", "updated0")
	requireKeyExists(t, cache, "key0", "updated0")
	requireCacheItems(t, cache, []string{"value2", "updated11", "updated0"})
}

func TestCache_Del_KeyNotExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	cache.Del("key0")
	requireKeyNotExists(t, cache, "key0")
}

func TestCache_Del_KeyExists(t *testing.T) {
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Delete middle
	cache.Del("key1")
	requireKeyNotExists(t, cache, "key1")
	requireCacheItems(t, cache, []string{"value0", "value2", "value3"})

	// Delete first
	cache.Del("key0")
	requireKeyNotExists(t, cache, "key0")
	requireCacheItems(t, cache, []string{"value2", "value3"})

	// Delete tail
	cache.Del("key3")
	requireKeyNotExists(t, cache, "key3")
	requireCacheItems(t, cache, []string{"value2"})

	// Delete tail
	cache.Del("key2")
	requireKeyNotExists(t, cache, "key2")
	requireCacheItems(t, cache, []string{})
}

func TestCache_GetOrRefresh_KeyNotExists(t *testing.T) {
	calls := atomic.Int32{}
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())

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
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
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
	cache := New[string, string](context.Background(), 0, NewNopMetrics())
	cache.Set("key0", "value0")
	// For testing purpose only
	cache.ttl = time.Second

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
	cache := New[string, string](context.Background(), time.Second, NewNopMetrics())
	actual, err := cache.GetOrRefresh("key0", func() (string, error) {
		calls.Add(1)
		return "", originErr
	})

	require.Equal(t, int32(1), calls.Load())
	require.ErrorIs(t, err, originErr)
	require.Empty(t, actual)
}

func TestCache_GetOrRefresh_RefreshFailed_Concurrent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := atomic.Int32{}
	cache := New[string, string](ctx, time.Second, NewNopMetrics())
	done := cache.SchedulePurge(time.Millisecond)

	wg := sync.WaitGroup{}
	wg.Add(3)

	mtx1 := sync.Mutex{}
	mtx1.Lock()
	mtx2 := sync.Mutex{}
	mtx2.Lock()
	mtx3 := sync.Mutex{}
	mtx3.Lock()
	var originErr = fmt.Errorf("some error")
	go func() { // First request will get an error on refresh call
		mtx1.Lock()
		defer mtx1.Unlock()
		defer wg.Done()

		_, err := cache.GetOrRefresh("key0", func() (string, error) {
			mtx2.Unlock()
			time.Sleep(10 * time.Millisecond)

			calls.Add(1)
			return "", originErr
		})
		require.ErrorIs(t, err, originErr)
	}()

	go func() { // Second request will get new value on refresh call without error
		mtx2.Lock()
		defer mtx2.Unlock()
		defer wg.Done()

		value, err := cache.GetOrRefresh("key0", func() (string, error) {
			mtx3.Unlock()
			time.Sleep(10 * time.Millisecond)
			calls.Add(1)
			return "value2", nil
		})
		require.NoError(t, err)
		require.Equal(t, "value2", value)
	}()

	go func() { // Third request will get value
		mtx3.Lock()
		defer mtx3.Unlock()
		defer wg.Done()

		value, err := cache.GetOrRefresh("key0", func() (string, error) {
			panic("should never be called")
		})
		require.NoError(t, err)
		require.Equal(t, "value2", value)
	}()

	mtx1.Unlock()
	wg.Wait()
	require.Equal(t, int32(2), calls.Load())

	cancel()
	<-done
}

func TestCache_Purge_Manually(t *testing.T) {
	cache := New[string, string](context.Background(), time.Nanosecond, NewNopMetrics())
	cache.Set("key0", "value0")
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	// These items should be created with ttl 3 Seconds
	cache.ttl = time.Second
	cache.Set("key3", "value3")
	cache.Set("key4", "value4")

	// Sleep a nanosecond to avoid flaky test on expiration key0, key1, key2.
	time.Sleep(time.Nanosecond)

	// After a nanosecond values should become expired.
	requireKeyNotExists(t, cache, "key0")
	requireKeyNotExists(t, cache, "key1")
	requireKeyNotExists(t, cache, "key2")

	// We don't schedule automatically purge, so items should be sill in the map.
	requireCacheItems(t, cache, []string{"value0", "value1", "value2", "value3", "value4"})

	// Call purge manually will remove expired items from map.
	cache.Purge()

	// Check rest not expired keys and values
	requireKeyExists(t, cache, "key3", "value3")
	requireKeyExists(t, cache, "key4", "value4")
	requireCacheItems(t, cache, []string{"value3", "value4"})
}
