package xclient

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
	"time"
)

type SelectMode int

const (
	RandomSelect SelectMode = iota
	RoundRobinSelect
)

type Discovery interface {
	Refresh() error
	Get(mode SelectMode) (string, error)
	GetAll() ([]string, error)
	Update(servers []string) error // server mean rpcAddr:  ex rpc@13.123.35.12
}

type MultiServerDiscovery struct {
	r       *rand.Rand
	mu      sync.Mutex
	servers []string
	index   int
}

func NewMultiServerDiscovery(server []string) *MultiServerDiscovery {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	index := r.Intn(math.MaxInt32 - 1)
	multiServerDiscovery := &MultiServerDiscovery{
		servers: server,
		r:       r,
		index:   index,
	}
	return multiServerDiscovery
}

var _ Discovery = (*MultiServerDiscovery)(nil)

func (d *MultiServerDiscovery) Refresh() error {
	// multiServerDiscovery ignore this method
	return nil
}

func (d *MultiServerDiscovery) Update(servers []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.servers = servers
	return nil
}

func (d *MultiServerDiscovery) Get(mode SelectMode) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	n := len(d.servers)
	if n == 0 {
		return "", fmt.Errorf("rpc discovery: not available discovery")
	}
	switch mode {
	case RandomSelect:
		return d.servers[d.r.Intn(n)], nil
	case RoundRobinSelect:
		server := d.servers[d.index]
		d.index++
		return server, nil
	default:
		return "", fmt.Errorf("rpc discovery: not support select mode")
	}
}

func (d *MultiServerDiscovery) GetAll() ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	servers := make([]string, len(d.servers), len(d.servers))
	copy(servers, d.servers)
	return servers, nil
}
