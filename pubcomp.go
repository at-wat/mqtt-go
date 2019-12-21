package mqtt

type PubComp struct {
	ID uint16
}

func (p *PubComp) parse(flag byte, contents []byte) *PubComp {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
