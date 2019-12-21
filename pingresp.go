package mqtt

type PingResp struct {
}

func (p *PingResp) parse(flag byte, contents []byte) *PingResp {
	return p
}
