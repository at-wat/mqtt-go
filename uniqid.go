package mqtt

import (
	"math/rand"
	"sync/atomic"
	"time"
)

var (
	idLast     uint32
	idInterval uint32
)

func init() {
	rand.Seed(time.Now().UnixNano())

	idLast = uint32(rand.Int31n(0xFFFF))
	idInterval = uint32(rand.Int31n(15) + 1)
}

func newID() uint16 {
	return uint16(atomic.AddUint32(&idLast, idInterval))
}
