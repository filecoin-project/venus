package messager

import (
	"time"

	"github.com/filecoin-project/venus/venus-shared/types"
)

type NodeType int

const (
	_ NodeType = iota
	FullNode
	LightNode
)

type Node struct {
	ID types.UUID

	Name      string
	URL       string
	Token     string
	Type      NodeType
	CreatedAt time.Time
	UpdatedAt time.Time
}
