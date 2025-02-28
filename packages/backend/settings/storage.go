package settings

// StorageBackend is a settings storage backend.
type StorageBackend interface {
	GetServer() (*Server, error)
	SaveServer(*Server) error
}

// Storage is a settings storage.
type Storage struct {
	back StorageBackend
}

// NewStorage creates a settings storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{back: back}
}

// GetServer wraps StorageBackend.GetServer.
func (s *Storage) GetServer() (*Server, error) {
	return s.back.GetServer()
}

// SaveServer wraps StorageBackend.SaveServer and adds some verification.
func (s *Storage) SaveServer(ser *Server) error {
	ser.Clean()
	return s.back.SaveServer(ser)
}
