package models

import (
	"errors"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
	"log"
	"strings"
)

var (
	ErrDuplicatedContactEmail = errors.New("duplicated contact email")
	ErrProfileNotFound        = errors.New("profile not found")
)

type Profile struct {
	ID                uint   `gorm:"primarykey"`
	User              string `gorm:"column:user;size:254;uniqueIndex;not null"`
	Nickname          string `gorm:"column:nickname;size:64"`
	Intro             string `gorm:"column:intro;size:256"`
	LangCode          string `gorm:"column:lang_code;size:50"`
	LoginID           string `gorm:"column:login_id;uniqueIndex;size:225"`
	ContactEmail      string `gorm:"column:contact_email;uniqueIndex;size:225"`
	Institution       string `gorm:"column:institution;size:225;index;default:''"`
	ListInAddressBook bool   `gorm:"column:list_in_address_book;index;default:false"`
}

func (Profile) TableName() string {
	return "profile_profile"
}

type ProfileManager struct {
	db *gorm.DB
}

func NewProfileManager(db *gorm.DB) *ProfileManager {
	return &ProfileManager{db: db}
}

func (m *ProfileManager) AddOrUpdate(username, nickname, intro, langCode,
	loginID, contactEmail, institution string, listInAddressBook bool) (*Profile, error) {

	var profile Profile
	if err := m.db.Where("user = ?", username).First(&profile).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		profile = Profile{User: username}
	}

	if nickname != "" {
		profile.Nickname = strings.TrimSpace(nickname)
	}
	if intro != "" {
		profile.Intro = intro
	}
	if langCode != "" {
		profile.LangCode = langCode
	}
	if loginID != "" {
		profile.LoginID = strings.TrimSpace(loginID)
	}
	if contactEmail != "" {
		profile.ContactEmail = strings.TrimSpace(contactEmail)
	}
	if institution != "" {
		profile.Institution = strings.TrimSpace(institution)
	}
	profile.ListInAddressBook = listInAddressBook

	if err := m.db.Save(&profile).Error; err != nil {
		if isDuplicateError(err) {
			return nil, ErrDuplicatedContactEmail
		}
		return nil, err
	}
	return &profile, nil
}

func (m *ProfileManager) UpdateContactEmail(username, contactEmail string) (*Profile, error) {
	var profile Profile
	if err := m.db.Where("user = ?", username).First(&profile).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.Printf("Profile for user %s does not exist", username)
			return nil, nil
		}
		return nil, err
	}

	profile.ContactEmail = strings.TrimSpace(contactEmail)
	if err := m.db.Save(&profile).Error; err != nil {
		if isDuplicateError(err) {
			return nil, ErrDuplicatedContactEmail
		}
		return nil, err
	}
	return &profile, nil
}

func (m *ProfileManager) GetProfileByUser(username string) (*Profile, error) {
	klog.Infof("~~~Debug log: begin to query info of user %q", username)

	var profiles []Profile
	if err := m.db.Find(&profiles).Error; err != nil {
		klog.Errorf("~~~Debug log: database query failed: %v", err)
		return nil, err
	}

	klog.Infof("~~~Debug log: totally got %d records", len(profiles))

	trimmedInput := strings.TrimSpace(username)

	for _, p := range profiles {
		klog.Infof("~~~Debug log: dealing record ID=%d, user=%q (length=%d), username=%s (length=%d)",
			p.ID, p.User, len(p.User), username, len(username))

		trimmedUser := strings.TrimSpace(p.User)

		if trimmedUser == trimmedInput {
			klog.Infof("~~~Debug log: match successfully ID=%d, user=%q", p.ID, p.User)
			return &p, nil
		} else {
			klog.Infof("~~~Debug log: match failed ID=%d, wanted=%q, actually=%q",
				p.ID, trimmedInput, trimmedUser)
		}
	}

	klog.Warning("~~~Debug log: record not found")
	return nil, ErrProfileNotFound
}

// TODO: have tried a lot of filter query condition, don't know why always "record not found". dealt next.
//func (m *ProfileManager) GetProfileByUser(username string) (*Profile, error) {
//	var profile Profile
//	err := m.db.Where("user = ?", username).First(&profile).Error
//	if errors.Is(err, gorm.ErrRecordNotFound) {
//		return nil, ErrProfileNotFound
//	}
//	return &profile, err
//}

func (m *ProfileManager) GetProfileByContactEmail(email string) (*Profile, error) {
	var profiles []Profile
	if err := m.db.Where("contact_email = ?", email).Find(&profiles).Error; err != nil {
		return nil, err
	}

	if len(profiles) > 1 {
		log.Printf("Warning: Repeated contact email %s", email)
	}
	if len(profiles) == 0 {
		return nil, nil
	}
	return &profiles[0], nil
}

func (m *ProfileManager) GetContactEmailByUser(username string) string {
	profile, err := m.GetProfileByUser(username)
	if err != nil || profile == nil || profile.ContactEmail == "" {
		return username
	}
	return profile.ContactEmail
}

func (m *ProfileManager) GetUsernameByLoginID(loginID string) (string, error) {
	if loginID == "" {
		return "", nil
	}

	var profile Profile
	err := m.db.Where("login_id = ?", loginID).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return profile.User, err
}

func (m *ProfileManager) GetUsernameByContactEmail(email string) (string, error) {
	if email == "" {
		return "", nil
	}

	var profile Profile
	err := m.db.Where("contact_email = ?", email).First(&profile).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", nil
	}
	return profile.User, err
}

func (m *ProfileManager) ConvertLoginStrToUsername(loginStr string) string {
	username, _ := m.GetUsernameByLoginID(loginStr)
	if username == "" {
		username, _ = m.GetUsernameByContactEmail(loginStr)
		if username == "" {
			return loginStr
		}
	}
	return username
}

func (m *ProfileManager) GetUserLanguage(username string) string {
	profile, err := m.GetProfileByUser(username)
	if err != nil || profile.LangCode == "" {
		return "en"
	}
	return profile.LangCode
}

func (m *ProfileManager) DeleteProfileByUser(username string) error {
	return m.db.Where("user = ?", username).Delete(&Profile{}).Error
}

func (m *ProfileManager) Filter(conditions map[string]interface{}) *gorm.DB {
	return m.db.Where(conditions)
}

func isDuplicateError(err error) bool {
	return strings.Contains(err.Error(), "Duplicate entry") ||
		strings.Contains(err.Error(), "UNIQUE constraint failed")
}
