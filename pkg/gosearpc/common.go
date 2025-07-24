package gosearpc

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
