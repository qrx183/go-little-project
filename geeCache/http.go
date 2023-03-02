package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"
)

const baseFilePath = "/_geeCache/"

type HTTPPool struct {
	self     string // 该服务的路径
	basePath string // 同一类服务的统一前缀
}

func NewHTTPPool(self string) *HTTPPool {
	return &HTTPPool{
		self:     self,
		basePath: baseFilePath,
	}
}

func (h *HTTPPool) Log(format string, value ...interface{}) {
	log.Printf("[Server %s] %s", h.self, fmt.Sprintf(format, value))
}

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

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")

	w.Write(bytes.byteSlice())
}
