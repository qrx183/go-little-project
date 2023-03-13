package registry

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

type GeeRegistry struct {
	timeout time.Duration
	mu      sync.Mutex
	servers map[string]*ServerItem
}

type ServerItem struct {
	Addr  string
	Start time.Time
}

const (
	defaultPath    = "/_geerpc_/registry"
	defaultTimeOut = 5 * time.Minute
)

func New(timeout time.Duration) *GeeRegistry {
	return &GeeRegistry{
		timeout: timeout,
		servers: make(map[string]*ServerItem),
	}
}

var DefaultGeeRegistry = New(defaultTimeOut)

func (r *GeeRegistry) putServer(addr string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.servers[addr]

	if s == nil {
		r.servers[addr] = &ServerItem{
			Addr:  addr,
			Start: time.Now(),
		}
	} else {
		s.Start = time.Now()
	}
}

func (r *GeeRegistry) aliveServers() []string {
	r.mu.Lock()
	defer r.mu.Unlock()

	var aliveServers []string

	for addr, s := range r.servers {
		if r.timeout == 0 || s.Start.Add(r.timeout).After(time.Now()) {
			aliveServers = append(aliveServers, addr)
		} else {
			delete(r.servers, addr)
		}
	}
	sort.Strings(aliveServers)
	return aliveServers
}

func (r *GeeRegistry) ServeHTTP(w http.ResponseWriter, req *http.Request) {

	switch req.Method {
	case "GET":
		w.Header().Set("X-GeeRpc-Servers", strings.Join(r.aliveServers(), ","))
	case "POST":
		addr := w.Header().Get("X-GeeRpc-Server")
		if addr == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.putServer(addr)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (r *GeeRegistry) HandleHTTP(registerPath string) {
	http.Handle(registerPath, r)
	log.Println("rpc registry path:", registerPath)
}

func HandleHTTP() {
	DefaultGeeRegistry.HandleHTTP(defaultPath)
}

func sendHeartbeat(registry, addr string) error {
	log.Println(addr, "send heart beat to registry ", registry)

	httpClient := &http.Client{}

	req, _ := http.NewRequest("POST", registry, nil)
	req.Header.Set("X-GeeRpc-Server", addr)
	if _, err := httpClient.Do(req); err != nil {
		log.Println("rpc server: heart beat err: ", err)
		return err
	}
	return nil
}

func HeartBeat(registry, addr string, duration time.Duration) {
	if duration == 0 {

		duration = defaultTimeOut - time.Duration(1)*time.Minute
	}
	var err error
	err = sendHeartbeat(registry, addr)
	go func() {
		t := time.NewTicker(duration)

		for err == nil {
			<-t.C
			err = sendHeartbeat(registry, addr)
		}
	}()
}
