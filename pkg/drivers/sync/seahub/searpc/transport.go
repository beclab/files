package searpc

type SearpcTransportInterface interface {
	Connect()
	Send(serviceName string, requestStr string) string
}

type SearpcTransport struct{}

func (t *SearpcTransport) Connect() {
	panic("Connect method must be implemented by concrete transport type")
}

func (t *SearpcTransport) Send(serviceName string, requestStr string) string {
	panic("Send method must be implemented by concrete transport type")
}
