package main

import (
	"fmt"
	"sync"
	"sync/atomic"

	lru "github.com/hashicorp/golang-lru/v2"
)

const lruCacheSize = 1 * 1024 * 1024

type Actor string

type Delta struct {
	Actor Actor
	Value map[string]int64
}

type gmap struct {
	me    Actor
	state map[Actor]*lru.Cache[string, *atomic.Int64]
	mu    sync.RWMutex
}

func newGMap(actor Actor) *gmap {
	cache, err := lru.New[string, *atomic.Int64](lruCacheSize)
	if err != nil {
		panic(fmt.Errorf("fail to allocate cache: %v", err))
	}
	return &gmap{
		me: actor,
		state: map[Actor]*lru.Cache[string, *atomic.Int64]{
			actor: cache,
		},
	}
}

func (g *gmap) Inc(key string) Delta {
	g.mu.Lock()
	defer g.mu.Unlock()
	val, ok := g.state[g.me].Get(key)
	if ok {
		val.Add(1)
	} else {
		val = new(atomic.Int64)
		val.Store(1)
		g.state[g.me].Add(key, val)
	}
	return Delta{
		Actor: g.me,
		Value: map[string]int64{
			key: val.Load(),
		},
	}
}

func (g *gmap) Value(key string) int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var ret int64
	for _, cache := range g.state {
		if value, ok := cache.Get(key); ok {
			ret += value.Load()
		}
	}
	return ret
}

func (g *gmap) Merge(state map[Actor]map[string]int64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for actor, kv := range state {
		if g.state[actor] == nil {
			var err error
			g.state[actor], err = lru.New[string, *atomic.Int64](lruCacheSize)
			if err != nil {
				panic(fmt.Errorf("fail to allocate cache: %v", err))
			}
		}
		for k, v := range kv {
			if existing, ok := g.state[actor].Peek(k); !ok {
				val := new(atomic.Int64)
				val.Store(v)
				g.state[actor].Add(k, val)
			} else if v > existing.Load() {
				existing.Store(v)
			}
		}
	}
}

func (g *gmap) ApplyDelta(delta Delta) {
	g.Merge(map[Actor]map[string]int64{
		delta.Actor: delta.Value,
	})
}

func (g *gmap) State() map[Actor]map[string]int64 {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result = make(map[Actor]map[string]int64)
	for actor, cache := range g.state {
		result[actor] = make(map[string]int64)
		for _, k := range cache.Keys() {
			if v, ok := cache.Peek(k); ok {
				result[actor][k] = v.Load()
			}
		}
	}
	return result
}
