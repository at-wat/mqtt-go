package mqtt

type pktPingResp struct {
}

func (p *pktPingResp) parse(flag byte, contents []byte) *pktPingResp {
	return p
}
