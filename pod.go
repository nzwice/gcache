package main

import (
	"bytes"
	"encoding/gob"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/memberlist"
)

type pod struct {
	name       string
	gossipAddr string
	kill       <-chan struct{}
	ml         *memberlist.Memberlist
	report     *time.Ticker
	lb         *roundRobinLb
	gmap       *gmap
	broadcasts *memberlist.TransmitLimitedQueue
}

func newPod(name string, gossipAddr string, lb *roundRobinLb, kill <-chan struct{}) *pod {
	cfg := memberlist.DefaultLANConfig()
	host, port := splitAddr(gossipAddr)
	cfg.PushPullInterval = 3 * time.Second      // full state sync
	cfg.GossipInterval = 200 * time.Millisecond // delta sync
	cfg.BindAddr = host
	cfg.BindPort = port
	cfg.Name = name
	ml, err := memberlist.Create(cfg)
	if err != nil {
		log.Println(err)
		return nil
	}
	po := &pod{
		name:       name,
		gossipAddr: gossipAddr,
		kill:       kill,
		ml:         ml,
		report:     time.NewTicker(time.Duration(reportSec) * time.Second),
		lb:         lb,
		gmap:       newGMap(Actor(name)),
		broadcasts: &memberlist.TransmitLimitedQueue{
			NumNodes: func() int {
				return ml.NumMembers()
			},
			RetransmitMult: 5,
		},
	}
	cfg.Delegate = po
	return po
}

func (p *pod) NodeMeta(limit int) []byte {
	return nil
}

func (p *pod) NotifyMsg(b []byte) {
	var delta Delta
	err := gob.NewDecoder(bytes.NewReader(b)).Decode(&delta)
	if err != nil {
		log.Println(p.name, "fail to decode delta", err)
		return
	}
	log.Println(p.name, "receive delta", delta)
	p.gmap.ApplyDelta(delta)
	log.Println(p.name, "after apply", p.gmap.State())
}

func (p *pod) GetBroadcasts(overhead, limit int) [][]byte {
	return p.broadcasts.GetBroadcasts(overhead, limit)
}

func (p *pod) LocalState(join bool) []byte {
	log.Println(p.name, "send local state...")
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(p.gmap.State())
	if err != nil {
		log.Println(p.name, "fail to encode state", err)
		return nil
	}
	return buf.Bytes()
}

func (p *pod) MergeRemoteState(buf []byte, join bool) {
	log.Println(p.name, "merge remote state...")
	var state map[Actor]map[string]int
	err := gob.NewDecoder(bytes.NewReader(buf)).Decode(&state)
	if err != nil {
		log.Println(p.name, "fail to decode state", err)
		return
	}
	p.gmap.Merge(state)
}

type envelop struct {
	Content []byte
}

func (e envelop) Invalidates(b memberlist.Broadcast) bool {
	return false
}

func (e envelop) Message() []byte {
	return e.Content
}

func (e envelop) Finished() {}

func (p *pod) Inc(key string) {
	delta := p.gmap.Inc(key)
	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(&delta)
	if err != nil {
		log.Println(p.name, "fail to encode delta", err)
		return
	}
	p.broadcasts.QueueBroadcast(&envelop{
		Content: buf.Bytes(),
	})
}

func (p *pod) Start() {
	log.Println(p.name, "starting pod...")
	if otherIp := p.lb.GossipAddr(); otherIp != "" {
		log.Println(p.name, "joining cluster with", otherIp)
		_, err := p.ml.Join([]string{otherIp})
		if err != nil {
			log.Println(err)
			return
		}
	}
	p.lb.Register(p)
	for {
		select {
		case <-p.report.C:
			var ar []interface{}
			for _, mb := range p.ml.Members() {
				ar = append(ar, mb.Address())
			}
			log.Println(p.name, "known pods", len(ar), ar)
		case <-p.kill:
			log.Println(p.name, "requested to stop")
			p.Shutdown()
			return
		}
	}
}

func (p *pod) Shutdown() {
	if err := p.ml.Shutdown(); err != nil {
		log.Println(err)
	}
	p.report.Stop()
}

func splitAddr(addr string) (host string, port int) {
	h, p, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalln(err)
	}
	pn, err := strconv.Atoi(p)
	if err != nil {
		log.Fatalln(err)
	}
	return h, pn
}
