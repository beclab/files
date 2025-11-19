package mediaprotocol

type MediaProtocol int

const (
	File MediaProtocol = iota
	Http
	Rtmp
	Rtsp
	Udp
	Rtp
	Ftp
)
