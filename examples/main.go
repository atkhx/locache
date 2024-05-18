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

	cache := locache.New[int64, *User](cacheTTL, locache.NewDefaultMetrics("users_cache"))
	cache.SchedulePurge(ctx, cachePurgeInterval)

	repo := NewUserRepository(cache)
	user, err := repo.GetUser(777)
	if err != nil {
		log.Println("failed to get user:", err)
		return
	}

	log.Println("got user:", user)
}
