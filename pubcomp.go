package mqtt

type pktPubComp struct {
	ID uint16
}

func (p *pktPubComp) parse(flag byte, contents []byte) *pktPubComp {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
