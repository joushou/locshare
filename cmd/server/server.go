package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"sync"
	"os"
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

	pubsubmapLock.RLock()
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
	pubsubmapLock.RUnlock()

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

	for {
		s, err := r.ReadString('\n')
		if err != nil {
			// That's an error!
			return
		}
		s = s[:len(s)-1]
		cmds := strings.Split(s, " ")
		switch cmds[0] {
		case "pub":
			if len(cmds) != 3 {
				return
			}
			log.Printf("pub: %s, %s", cmds[1], cmds[2])
			push(cmds[1], []byte(cmds[2]))
		case "sub":
			if len(cmds) != 2 {
				return
			}
			log.Printf("sub: %v", cmds[1])
			if subSet[cmds[1]] {
				log.Printf("already subscribed")
				continue
			}

			subSet[cmds[1]] = true
			go func(id string) {
				ph := get(id)
				defer ph.close()
				for {
					b := ph.fetch()
					if err := writerFunc(id, b); err != nil {
						return
					}
				}
			}(cmds[1])
		}
	}
}

func main() {
	l, err := net.Listen("tcp", os.Args[1])
	if err != nil {
		return
	}

	for {
		c, err := l.Accept()
		if err != nil {
			return
		}

		go handler(c)
	}
}
