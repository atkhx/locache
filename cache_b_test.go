package locache

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func BenchmarkCache_Get(b *testing.B) {
	type benchCase struct {
		bulletsCount   int
		itemsTTL       time.Duration
		readInterval   time.Duration
		writeInterval  time.Duration
		deleteInterval time.Duration
		purgeInterval  time.Duration
	}

	benchCases := map[string]benchCase{
		"case1": {
			bulletsCount:   10_000,
			itemsTTL:       1 * time.Second,
			readInterval:   1 * time.Nanosecond,
			writeInterval:  1 * time.Nanosecond,
			deleteInterval: 1 * time.Nanosecond,
			purgeInterval:  1 * time.Millisecond,
		},
		"case2": {
			bulletsCount:   10_000,
			itemsTTL:       1 * time.Millisecond,
			readInterval:   1 * time.Nanosecond,
			writeInterval:  1 * time.Nanosecond,
			deleteInterval: 1 * time.Nanosecond,
			purgeInterval:  1 * time.Millisecond,
		},
		"case3": {
			bulletsCount:   100,
			itemsTTL:       1 * time.Second,
			readInterval:   10 * time.Millisecond,
			writeInterval:  10 * time.Millisecond,
			deleteInterval: 10 * time.Millisecond,
			purgeInterval:  1 * time.Millisecond,
		},
	}

	schedule := func(ctx context.Context, interval time.Duration, fn func()) {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval + time.Duration(rand.Intn(1000))):
					fn()
				}
			}
		}()
	}

	for name, tc := range benchCases {
		b.Run(name, func(b *testing.B) {
			bullets := make([]string, 0, tc.bulletsCount)
			for i := 0; i < tc.bulletsCount; i++ {
				bullets = append(bullets, fmt.Sprintf("key-%d", i))
			}

			ctx, cancel := context.WithCancel(context.Background())
			cache := New[string, string](ctx, tc.itemsTTL)
			purgeDone := cache.SchedulePurge(tc.purgeInterval)

			schedule(ctx, tc.readInterval, func() { cache.Get(bullets[rand.Intn(tc.bulletsCount)]) })
			schedule(ctx, tc.readInterval, func() { cache.Set(bullets[rand.Intn(tc.bulletsCount)], fmt.Sprintf("val %d", time.Now().UnixNano())) })
			schedule(ctx, tc.readInterval, func() { cache.Delete(bullets[rand.Intn(tc.bulletsCount)]) })

			for i := 0; i < b.N; i++ {
				cache.Get(bullets[rand.Intn(tc.bulletsCount)])
			}

			cancel()
			<-purgeDone
		})
	}
}

func BenchmarkCache_GetOrRefresh(b *testing.B) {
	type benchCase struct {
		bulletsCount   int
		itemsTTL       time.Duration
		readInterval   time.Duration
		writeInterval  time.Duration
		deleteInterval time.Duration
		purgeInterval  time.Duration
	}

	benchCases := map[string]benchCase{
		"case1": {
			bulletsCount:   10_000,
			itemsTTL:       1 * time.Second,
			readInterval:   1 * time.Nanosecond,
			writeInterval:  1 * time.Nanosecond,
			deleteInterval: 1 * time.Nanosecond,
			purgeInterval:  1 * time.Millisecond,
		},
		"case2": {
			bulletsCount:   10_000,
			itemsTTL:       1 * time.Millisecond,
			readInterval:   1 * time.Nanosecond,
			writeInterval:  1 * time.Nanosecond,
			deleteInterval: 1 * time.Nanosecond,
			purgeInterval:  1 * time.Millisecond,
		},
		"case3": {
			bulletsCount:   100,
			itemsTTL:       1 * time.Second,
			readInterval:   10 * time.Millisecond,
			writeInterval:  10 * time.Millisecond,
			deleteInterval: 10 * time.Millisecond,
			purgeInterval:  1 * time.Millisecond,
		},
	}

	schedule := func(ctx context.Context, interval time.Duration, fn func()) {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				case <-time.After(interval + time.Duration(rand.Intn(1000))):
					fn()
				}
			}
		}()
	}

	for name, tc := range benchCases {
		b.Run(name, func(b *testing.B) {
			bullets := make([]string, 0, tc.bulletsCount)
			for i := 0; i < tc.bulletsCount; i++ {
				bullets = append(bullets, fmt.Sprintf("key-%d", i))
			}

			ctx, cancel := context.WithCancel(context.Background())
			cache := New[string, string](ctx, tc.itemsTTL)
			purgeDone := cache.SchedulePurge(tc.purgeInterval)

			schedule(ctx, tc.readInterval, func() { cache.Get(bullets[rand.Intn(tc.bulletsCount)]) })
			schedule(ctx, tc.readInterval, func() { cache.Set(bullets[rand.Intn(tc.bulletsCount)], fmt.Sprintf("val %d", time.Now().UnixNano())) })
			schedule(ctx, tc.readInterval, func() { cache.Delete(bullets[rand.Intn(tc.bulletsCount)]) })

			for i := 0; i < b.N; i++ {
				key := bullets[rand.Intn(tc.bulletsCount)]
				cache.GetOrRefresh(key, func() (string, error) {
					time.Sleep(10 * time.Millisecond)
					return fmt.Sprintf("val 1 %d", time.Now().UnixNano()), nil
				})
			}

			cancel()
			<-purgeDone
		})
	}
}
