package dlna

import (
	"strings"
)

type ContainerProfile struct {
	Type       DlnaProfileType
	Conditions []ProfileCondition
	Container  string
}

func NewContainerProfile() *ContainerProfile {
	return &ContainerProfile{
		Conditions: make([]ProfileCondition, 0),
	}
}

func SplitValue(value *string) []string {
	if value == nil || *value == "" {
		return []string{}
	}

	return strings.Split(*value, ",")
}

func (cp *ContainerProfile) ContainsContainer(container *string) bool {
	return ContainsContainer2(SplitValue(&cp.Container), container)
}

func ContainsContainer(profileContainers, inputContainer *string) bool {
	isNegativeList := false
	if profileContainers != nil && strings.HasPrefix(*profileContainers, "-") {
		isNegativeList = true
		*profileContainers = (*profileContainers)[1:]
	}

	return ContainsContainer3(SplitValue(profileContainers), isNegativeList, inputContainer)
}

func ContainsContainer2(profileContainers []string, inputContainer *string) bool {
	return ContainsContainer3(profileContainers, false, inputContainer)
}

func ContainsContainer3(profileContainers []string, isNegativeList bool, inputContainer *string) bool {
	if len(profileContainers) == 0 {
		// Empty profiles always support all containers/codecs
		return true
	}

	allInputContainers := SplitValue(inputContainer)

	for _, container := range allInputContainers {
		for _, profileContainer := range profileContainers {
			if strings.EqualFold(container, profileContainer) {
				return !isNegativeList
			}
		}
	}

	return isNegativeList
}
