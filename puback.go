package mqtt

type pktPubAck struct {
	ID uint16
}

func (p *pktPubAck) parse(flag byte, contents []byte) *pktPubAck {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
