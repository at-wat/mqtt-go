package mqtt

type pktPubComp struct {
	ID uint16
}

func (p *pktPubComp) parse(flag byte, contents []byte) *pktPubComp {
	_, p.ID = unpackUint16(contents)
	return p
}
