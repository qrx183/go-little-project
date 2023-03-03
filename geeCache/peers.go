package main

import pb "geeCache/geeCachePb"

// PeerPicker
// PickPeer:根据key去获取对应节点上的HTTP客户端
type PeerPicker interface {
	PickPeer(key string) (PeerGetter, bool)
}

// PeerGetter HTTP客户端
// Get:向该客户端对应的服务端节点上获取缓存值
type PeerGetter interface {
	Get(in *pb.Request, out *pb.Response) error
}
