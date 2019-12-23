package mqtt

import (
	"math/rand"
	"sync/atomic"
	"time"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func (c *BaseClient) initID() {
	atomic.StoreUint32(&c.idLast, uint32(rand.Int31n(0xFFFE))+1)
}

func (c *BaseClient) newID() uint16 {
	id := uint16(atomic.AddUint32(&c.idLast, 1))
	if id == 0 {
		return c.newID()
	}
	return id
}
