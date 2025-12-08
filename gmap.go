package main

import (
	"fmt"
	"sync"

	lru "github.com/hashicorp/golang-lru/v2"
)

const lruCacheSize = 1 * 1024 * 1024

type Actor string

type Delta struct {
	Actor Actor
	Value map[string]int
}

type gmap struct {
	me    Actor
	state map[Actor]*lru.Cache[string, int]
	mu    sync.RWMutex
}

func newGMap(actor Actor) *gmap {
	cache, err := lru.New[string, int](lruCacheSize)
	if err != nil {
		panic(fmt.Errorf("fail to allocate cache: %v", err))
	}
	return &gmap{
		me: actor,
		state: map[Actor]*lru.Cache[string, int]{
			actor: cache,
		},
	}
}

func (g *gmap) Inc(key string) Delta {
	g.mu.Lock()
	defer g.mu.Unlock()
	value, _ := g.state[g.me].Get(key)
	value++
	g.state[g.me].Add(key, value)
	return Delta{
		Actor: g.me,
		Value: map[string]int{
			key: value,
		},
	}
}

func (g *gmap) Value(key string) int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var ret int
	for _, cache := range g.state {
		value, _ := cache.Get(key)
		ret += value
	}
	return ret
}

func (g *gmap) Merge(state map[Actor]map[string]int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	for actor, kv := range state {
		if g.state[actor] == nil {
			var err error
			g.state[actor], err = lru.New[string, int](lruCacheSize)
			if err != nil {
				panic(fmt.Errorf("fail to allocate cache: %v", err))
			}
		}
		for k, v := range kv {
			if existing, ok := g.state[actor].Get(k); !ok || v > existing {
				g.state[actor].Add(k, v)
			}
		}
	}
}

func (g *gmap) ApplyDelta(delta Delta) {
	g.Merge(map[Actor]map[string]int{
		delta.Actor: delta.Value,
	})
}

func (g *gmap) State() map[Actor]map[string]int {
	g.mu.RLock()
	defer g.mu.RUnlock()
	var result = make(map[Actor]map[string]int)
	for actor, cache := range g.state {
		result[actor] = make(map[string]int)
		for _, k := range cache.Keys() {
			if v, ok := cache.Get(k); ok {
				result[actor][k] = v
			}
		}
	}
	return result
}
