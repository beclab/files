package dto

type NameValuePair struct {
	Name  string
	Value string
}

func NewNameValuePair(name, value string) *NameValuePair {
	return &NameValuePair{
		Name:  name,
		Value: value,
	}
}
