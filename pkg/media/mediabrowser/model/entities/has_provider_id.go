package entities

type IHasProviderIds interface {
	GetProviderIds() map[string]string
	SetProviderIds(map[string]string)
}
