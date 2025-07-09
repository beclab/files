package commands

type command struct{}

func NewCommand() *command {
	return &command{}
}
