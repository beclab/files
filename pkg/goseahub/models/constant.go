package models

import (
	"github.com/patrickmn/go-cache"
	"time"
)

var (
	GlobalProfileManager *ProfileManager

	NICKNAME_CACHE_TIMEOUT = 24 * time.Hour
	NICKNAME_CACHE_PREFIX  = "NICKNAME_"

	EMAIL_ID_CACHE_TIMEOUT = 24 * time.Hour
	EMAIL_ID_CACHE_PREFIX  = "EMAIL_ID_"

	CONTACT_CACHE_TIMEOUT = 24 * time.Hour
	CONTACT_CACHE_PREFIX  = "CONTACT_"

	contactCache  = cache.New(CONTACT_CACHE_TIMEOUT, 10*time.Minute)
	nicknameCache = cache.New(NICKNAME_CACHE_TIMEOUT, 10*time.Minute)
)
