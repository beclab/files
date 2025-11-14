package dlna

type ProfileCondition struct {
	Condition  ProfileConditionType
	Property   ProfileConditionValue
	Value      string
	IsRequired bool
}

func NewProfileCondition(condition ProfileConditionType, property ProfileConditionValue, value string) *ProfileCondition {
	return NewProfileConditionWithRequired(condition, property, value, true)
}

func NewProfileConditionWithRequired(condition ProfileConditionType, property ProfileConditionValue, value string, isRequired bool) *ProfileCondition {
	return &ProfileCondition{
		Condition:  condition,
		Property:   property,
		Value:      value,
		IsRequired: isRequired,
	}
}
