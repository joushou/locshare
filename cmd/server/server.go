package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	pubsubmapLock sync.RWMutex
	pubsubmap     = make(map[string]*thing)
)

type thing struct {
	sync.RWMutex
	subs []*pubsub
}

func push(id string, data []byte) {
	pubsubmapLock.RLock()
	t := pubsubmap[id]
	pubsubmapLock.RUnlock()

	if t == nil {
		return
	}

	t.RLock()
	defer t.RUnlock()
	for _, sub := range t.subs {
		sub.push(data)
	}
}

func get(id string) *pubsub {
	p := &pubsub{
		id:        id,
		queueCond: sync.NewCond(&sync.Mutex{}),
	}

	pubsubmapLock.Lock()
	t := pubsubmap[id]
	if t == nil {
		t = &thing{
			subs: []*pubsub{p},
		}
		pubsubmap[id] = t
	} else {
		t.Lock()
		t.subs = append(t.subs, p)
		t.Unlock()
	}
	pubsubmapLock.Unlock()

	return p
}

func rm(p *pubsub) {
	pubsubmapLock.RLock()
	t := pubsubmap[p.id]
	pubsubmapLock.RUnlock()

	if t == nil {
		return
	}

	t.Lock()
	defer t.Unlock()
	for i := range t.subs {
		if t.subs[i] == p {
			t.subs = append(t.subs[:i], t.subs[i+1:]...)
			break
		}
	}

	if len(t.subs) == 0 {
		pubsubmapLock.Lock()
		delete(pubsubmap, p.id)
		pubsubmapLock.Unlock()
	}

	return
}

type pubsub struct {
	sync.RWMutex
	id        string
	queue     [][]byte
	queueCond *sync.Cond

	unwanted bool
}

func (p *pubsub) fetch() []byte {
	p.queueCond.L.Lock()
	defer p.queueCond.L.Unlock()
	for len(p.queue) == 0 && p.unwanted == false {
		p.queueCond.Wait()
	}

	if p.unwanted {
		return nil
	}

	b := p.queue[0]
	p.queue = p.queue[1:]
	return b
}

func (p *pubsub) push(b []byte) {
	p.queueCond.L.Lock()
	defer p.queueCond.L.Unlock()
	p.queue = append(p.queue, b)
	p.queueCond.Signal()
}

func (p *pubsub) close() {
	p.queueCond.L.Lock()
	p.queue = nil
	p.unwanted = true
	p.queueCond.Broadcast()
	p.queueCond.L.Unlock()

	rm(p)
}

func handler(c net.Conn) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("panic: %v", r)
		}
	}()

	r := bufio.NewReader(c)
	var once sync.Once
	closerFunc := func() {
		c.Close()
	}

	defer once.Do(closerFunc)

	subSet := make(map[string]bool)

	var writerLock sync.Mutex
	writerFunc := func(id string, b []byte) error {
		writerLock.Lock()
		defer writerLock.Unlock()
		_, err := fmt.Fprintf(c, "%s %s\n", id, b)
		if err != nil {
			once.Do(closerFunc)
		}
		return err
	}


	var handles []pubsub
	var close_next bool
	for {
		s, prefix, err := r.ReadLine()
		if err != nil {
			if err != io.EOF {
				log.Printf("error: %v", err)
			}
			goto done
		}
		if prefix {
			log.Printf("too much data from client, terminating")
			goto done
		}
		cmds := strings.Split(string(s), " ")
		switch cmds[0] {
		case "close_next":
			close_next = true
		case "close":
			goto done
		case "pub":
			if len(cmds) != 3 {
				goto done
			}
			log.Printf("pub: %s, %s", cmds[1], cmds[2])
			push(cmds[1], []byte(cmds[2]))

			if close_next {
				goto done
			}
		case "sub":
			if len(cmds) != 2 {
				goto done
			}
			log.Printf("sub: %v", cmds[1])
			if subSet[cmds[1]] {
				continue
			}

			subSet[cmds[1]] = true
			ph := get(id)
			handles = append(handles, ph)
			go func(ph pubsub) {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("panic: %v", r)
					}
				}()

				for {
					b := ph.fetch()
					if err := writerFunc(id, b); err != nil {
						return
					}
				}
			}(ph)
		}
	}

done:
	for _, h := range handles {
		h.close()
	}
}

// AcceptLimiter implements rate limiting, as this is publicly accessible!
// Within a 100ms window, only 16 users may connect. If connections occur at a
// higher rate, sleep so that the window is enforced. While this protects the
// server against load and abuse, it does not guarantee service availability,
// as real users will also be met with a sleep during a DoS, potentially
// timing out.
type AcceptLimiter struct {
	net.Listener
	times []time.Time
	idx   int
	gap   time.Duration
}

func NewAcceptLimiter(l net.Listener) net.Listener {
	al := AcceptLimiter{
		Listener: l,
		times:    make([]time.Time, 16),
		gap:      100 * time.Millisecond,
	}

	for i := range al.times {
		al.times[i] = time.Now()
	}

	return &al
}

func (al *AcceptLimiter) Accept() (net.Conn, error) {
	n := time.Now()
	al.times[al.idx] = n
	al.idx = (al.idx + 1) & 15
	t := al.times[al.idx].Add(al.gap)
	if t.After(n) {
		time.Sleep(t.Sub(n))
	}

	return al.Listener.Accept()
}

func main() {
	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		return
	}

	l = NewAcceptLimiter(l)

	for {
		c, err := l.Accept()
		if err != nil {
			return
		}

		go handler(c)
	}
}
