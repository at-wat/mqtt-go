package mqtt

type PubAck struct {
	ID uint16
}

func (p *PubAck) parse(flag byte, contents []byte) *PubAck {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
