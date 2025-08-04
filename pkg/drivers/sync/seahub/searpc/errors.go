package searpc

type SearpcError struct {
	Msg string
}

func NewSearpcError(msg string) *SearpcError {
	return &SearpcError{
		Msg: msg,
	}
}

func (e *SearpcError) Error() string {
	return e.Msg
}

type NetworkError struct {
	Msg string
}

func NewNetworkError(msg string) *NetworkError {
	return &NetworkError{
		Msg: msg,
	}
}

func (e *NetworkError) Error() string {
	return e.Msg
}
