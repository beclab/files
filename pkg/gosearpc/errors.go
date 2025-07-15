package gosearpc

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
