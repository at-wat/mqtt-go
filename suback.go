package mqtt

type SubAck struct {
	ID    uint16
	Codes []subscribeFlag
}

func (p *SubAck) parse(flag byte, contents []byte) *SubAck {
	p.ID = uint16(contents[0])<<8 | uint16(contents[1])
	for _, c := range contents[2:] {
		p.Codes = append(p.Codes, subscribeFlag(c))
	}
	return p
}
