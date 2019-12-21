package mqtt

type pktPubRec struct {
	ID uint16
}

func (p *pktPubRec) parse(flag byte, contents []byte) *pktPubRec {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
