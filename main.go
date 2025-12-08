package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var reportSec int

func main() {

	var podCount int
	var startingPort int
	var delaySec int
	var jitterSec int
	var lbAddr string

	flag.IntVar(&podCount, "count", 2, "the number of expected pods in the cluster")
	flag.IntVar(&reportSec, "report", 3, "periodic seconds for a pod to report its known cluster state")
	flag.IntVar(&startingPort, "port", 8100, "the starting gossip port, the ith pod will listen gossip port on (starting + i)")
	flag.IntVar(&delaySec, "delay", 5, "delay seconds before starting a pod, the ith pod will be delayed within (delay * i) seconds randomly")
	flag.IntVar(&jitterSec, "jitter", 5, "extra jitter seconds to delay a pod")
	flag.StringVar(&lbAddr, "lb", ":8098", "the load balancer http address")
	flag.Parse()

	var wg sync.WaitGroup
	var scheduler []*time.Timer

	// round-robin lb to load balance pods
	wg.Add(1)
	var lb = newLb(lbAddr, func() {
		wg.Done()
	})
	go lb.Start()

	// to stop all pods
	done := make(chan struct{})

	// run all pods, lets them communicate
	for i := 0; i < podCount; i++ {
		ind := i
		port := startingPort + ind
		addr := fmt.Sprintf("127.0.0.1:%v", port)
		joinSec := rand.Intn(delaySec + delaySec*i + jitterSec)
		tm := time.AfterFunc(time.Duration(joinSec)*time.Second, func() {
			wg.Add(1)
			defer wg.Done()
			po := newPod(podName(ind), addr, lb, done)
			po.Start()
		})
		scheduler = append(scheduler, tm)
	}

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	go func() {
		v := <-sig
		log.Println("receiving signal", v)
		log.Println("stop existing scheduler...")
		for _, tm := range scheduler {
			tm.Stop()
		}
		log.Println("stopping lb...")
		lb.Shutdown()
		log.Println("stopping pods...")
		close(done)
		log.Println("byte byte...")
	}()
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	wg.Wait()
}

func podName(ind int) string {
	return fmt.Sprintf("pod-#%v", ind)
}
