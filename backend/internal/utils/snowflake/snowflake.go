package snowflake

import (
	"errors"

	sf "github.com/bwmarrin/snowflake"
)

// Node 雪花算法发号器节点，封装 bwmarrin/snowflake 库
type Node struct {
	node *sf.Node // bwmarrin/snowflake 底层节点，内部自带互斥锁，并发安全
}

// NewNode 创建雪花算法发号器节点
// workerID 为节点标识，取值范围 [0, 1023]，多实例部署时各节点必须唯一，避免ID冲突
func NewNode(workerID int64) (*Node, error) {
	n, err := sf.NewNode(workerID)
	if err != nil {
		return nil, err
	}
	return &Node{node: n}, nil
}

// NextID 生成下一个全局唯一ID（int64）
func (n *Node) NextID() (int64, error) {
	if n == nil || n.node == nil {
		return 0, errors.New("snowflake node is not initialized")
	}
	return n.node.Generate().Int64(), nil
}

// NextIDStr 生成下一个全局唯一ID的十进制字符串形式
func (n *Node) NextIDStr() (string, error) {
	if n == nil || n.node == nil {
		return "", errors.New("snowflake node is not initialized")
	}
	return n.node.Generate().String(), nil
}
