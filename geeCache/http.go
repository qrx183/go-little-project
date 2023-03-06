package main

import (
	"fmt"
	"geeCache/consistenthash"
	pb "geeCache/geeCachePb"
	"github.com/golang/protobuf/proto"
	"io/ioutil"
	"log"
	"net/http"
	url2 "net/url"
	"strings"
	"sync"
)

const (
	baseFilePath    = "/_geeCache/"
	defaultReplicas = 50
)

type HTTPPool struct {
	self        string // 该服务的路径
	basePath    string // 同一类服务的统一前缀
	mu          sync.Mutex
	peers       *consistenthash.Map
	httpGetters map[string]*httpGetter
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: baseFilePath,
	}
}

func (h *HTTPPool) Set(addrs ...string) {
	// 在该服务上设置可选的服务器节点
	mu.Lock()

	defer mu.Unlock()
	h.peers = consistenthash.New(defaultReplicas, nil)
	h.peers.Add(addrs...)
	h.httpGetters = make(map[string]*httpGetter, len(addrs))
	for _, add := range addrs {
		h.httpGetters[add] = &httpGetter{baseURL: add + baseFilePath}
	}

}

func (h *HTTPPool) PickPeer(key string) (PeerGetter, bool) {

	// 首先获取远程真实节点,然后获取真实节点对应的HTTP客户端
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.peers != nil {
		// 如果远程节点就是节点本身，没必要获取远程节点，在本身节点上就可以拿到缓存
		// 这里的peer != h.self不仅是排除获取远程节点,还保证了每个key都是在经过一致性哈希选择对应的节点以后再在该节点上进行缓存更新
		// 这样就保证了相同的key每次经过一致性哈希以后都会从同一个远程节点那里进行获取  妙!太妙了!
		if peer := h.peers.Get(key); peer != "" && peer != h.self {
			h.Log("Pick peer %s", peer)
			return h.httpGetters[peer], true
		}
	}
	return nil, false
}

func (h *HTTPPool) Log(format string, value ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, value))
}

// ServeHTTP
// http服务端
func (h *HTTPPool) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if !strings.HasPrefix(req.URL.Path, h.basePath) {
		panic("HTTPPool serving unexpected path " + req.URL.Path)
	}

	path := req.URL.Path

	parts := strings.SplitN(path[len(h.basePath):], "/", 2)

	groupName := parts[0]
	key := parts[1]

	group := GetGroups(groupName)

	if group == nil {
		http.Error(w, "no this group "+groupName, http.StatusNotFound)
		return
	}

	bytes, err := group.Get(key)

	// 利用proto对响应内容进行编码,从而提升传输效率
	body, err := proto.Marshal(&pb.Response{Value: bytes.byteSlice()})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	w.Write(body)
}

// httpGetter
// http客户端
type httpGetter struct {
	baseURL string
}

func (p *httpGetter) Get(in *pb.Request, out *pb.Response) error {
	url := fmt.Sprintf("%v%v/%v", p.baseURL, url2.QueryEscape(in.Group), url2.QueryEscape(in.Key))

	res, err := http.Get(url)

	defer res.Body.Close()
	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned: %v", res.Status)
	}

	bytes, err := ioutil.ReadAll(res.Body)

	// 对bytes进行解码
	if err := proto.Unmarshal(bytes, out); err != nil {
		return fmt.Errorf("decoding response body: %v", err)
	}
	if err != nil {
		return fmt.Errorf("reading response body: %v", err)
	}

	return nil
}
