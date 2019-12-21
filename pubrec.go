package mqtt

type PubRec struct {
	ID uint16
}

func (p *PubRec) parse(flag byte, contents []byte) *PubRec {
	p.ID = (uint16(contents[0]) << 8) | uint16(contents[1])
	return p
}
