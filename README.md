## locache

This library provides a simple implementation of in-memory local cache with TTL.

### Overview

This library was created to solve a specific problem: reducing the number of repeatedly executed operations within a time window by locking on the key.

### Key Features

- Using generics to create cache for required key and data structure. 
- Configurable TTL and purge interval.
- Two levels of locks (the entire key map and individual item locks) minimize the impact of parallel operations.
- The cache size is not limited.

### Installation

To install, use:

```sh
go get github.com/atkhx/locache
```

### Usage

Check the [example](./examples/main.go).

```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/atkhx/locache"
)

type (
	User struct {
		ID   int64
		Name string
	}
	UserCache interface {
		GetOrRefresh(id int64, refresh func() (*User, error)) (*User, error)
	}
	UsersRepository struct {
		cache UserCache
	}
)

func NewUserRepository(cache UserCache) *UsersRepository {
	return &UsersRepository{cache}
}

func (r *UsersRepository) GetUser(id int64) (*User, error) {
	return r.cache.GetOrRefresh(id, func() (*User, error) {
		// perform some slow operation for getting a User data
		return &User{ID: id, Name: "John"}, nil
	})
}

const (
	cacheTTL           = time.Second
	cachePurgeInterval = 100 * time.Millisecond
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cache := locache.New[int64, *User](ctx, cacheTTL, locache.NewDefaultMetrics("users_cache"))
	cache.SchedulePurge(cachePurgeInterval)

	repo := NewUserRepository(cache)
	user, err := repo.GetUser(777)
	if err != nil {
		log.Println("failed to get user:", err)
		return
	}

	log.Println("got user:", user)
}

```

### Contributing

Feel free to contribute to this project by opening issues or submitting pull requests.

### License

This project is licensed under the MIT License.