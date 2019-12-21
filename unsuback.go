package mqtt

type UnsubAck struct {
	ID uint16
}

func (p *UnsubAck) parse(flag byte, contents []byte) *UnsubAck {
	p.ID = uint16(contents[0])<<8 | uint16(contents[1])
	return p
}
