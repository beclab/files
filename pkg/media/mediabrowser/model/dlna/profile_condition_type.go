package dlna

type ProfileConditionType int

const (
	Equals ProfileConditionType = iota
	NotEquals
	LessThanEqual
	GreaterThanEqual
	EqualsAny
)
