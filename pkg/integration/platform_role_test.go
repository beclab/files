package integration

import (
	"testing"

	"files/pkg/models"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func testUser(name, role string) *models.User {
	return &models.User{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Annotations: map[string]string{"bytetrade.io/owner-role": role},
		},
	}
}

func TestPlatformRoleHelpers(t *testing.T) {
	svc := &integration{users: []*models.User{
		testUser("owner1", PlatformRoleOwner),
		testUser("admin1", PlatformRoleAdmin),
		testUser("normal1", PlatformRoleNormal),
	}}

	roleCases := map[string]struct {
		wantRole string
		wantOK   bool
	}{
		"owner1":  {PlatformRoleOwner, true},
		"admin1":  {PlatformRoleAdmin, true},
		"normal1": {PlatformRoleNormal, true},
		"ghost":   {"", false},
		"":        {"", false},
	}
	for name, want := range roleCases {
		role, ok := svc.GetPlatformRole(name)
		if role != want.wantRole || ok != want.wantOK {
			t.Errorf("GetPlatformRole(%q) = (%q,%v), want (%q,%v)", name, role, ok, want.wantRole, want.wantOK)
		}
		if got := svc.UserExists(name); got != want.wantOK {
			t.Errorf("UserExists(%q) = %v, want %v", name, got, want.wantOK)
		}
	}

	adminCases := map[string]bool{
		"owner1":  true,
		"admin1":  true,
		"normal1": false,
		"ghost":   false,
	}
	for name, want := range adminCases {
		if got := svc.IsPlatformAdmin(name); got != want {
			t.Errorf("IsPlatformAdmin(%q) = %v, want %v", name, got, want)
		}
	}
}
