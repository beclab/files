package session

import (
	"sort"
	"strings"
	"sync"

	"files/pkg/media/mediabrowser/controller/session"
	ms "files/pkg/media/mediabrowser/model/session"
)

type SessionManager struct {
	activeConnections sync.Map
}

/*
type SessionManager struct {
    logger                *log.Logger
    eventManager          EventManager
    userDataManager       UserDataManager
    config                ServerConfigurationManager
    libraryManager        LibraryManager
    userManager           UserManager
    musicManager          MusicManager
    dtoService            DtoService
    imageProcessor        ImageProcessor
    appHost               ServerApplicationHost
    deviceManager         DeviceManager
    mediaSourceManager    MediaSourceManager
    shutdownCallback      func()
}

func NewSessionManager(
    logger                *log.Logger,
    eventManager          EventManager,
    userDataManager       UserDataManager,
    config                ServerConfigurationManager,
    libraryManager        LibraryManager,
    userManager           UserManager,
    musicManager          MusicManager,
    dtoService            DtoService,
    imageProcessor        ImageProcessor,
    appHost               ServerApplicationHost,
    deviceManager         DeviceManager,
    mediaSourceManager    MediaSourceManager,
    hostApplicationLifetime HostApplicationLifetime,
) *SessionManager {
    sm := &SessionManager{
        logger:                logger,
        eventManager:          eventManager,
        userDataManager:       userDataManager,
        config:                config,
        libraryManager:        libraryManager,
        userManager:           userManager,
        musicManager:          musicManager,
        dtoService:            dtoService,
        imageProcessor:        imageProcessor,
        appHost:               appHost,
        deviceManager:         deviceManager,
        mediaSourceManager:    mediaSourceManager,
    }

    sm.shutdownCallback = hostApplicationLifetime.ApplicationStopping.Register(sm.onApplicationStopping)
    deviceManager.DeviceOptionsUpdated.Subscribe(sm.onDeviceManagerDeviceOptionsUpdated)

    return sm
}

func (sm *SessionManager) onDeviceManagerDeviceOptionsUpdated(sender interface{}, args EventArgs) {
    // Handle device options updated event
}

func (sm *SessionManager) onApplicationStopping(sender interface{}, args EventArgs) {
    // Handle application stopping event
}
*/

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func (m *SessionManager) AddSession(deviceId string, session *session.SessionInfo) {
	m.activeConnections.Store(deviceId, session)
}

func (m *SessionManager) GetSession(deviceId, client, version string) *session.SessionInfo {
	var foundSession *session.SessionInfo
	m.activeConnections.Range(func(key, value interface{}) bool {
		session := value.(*session.SessionInfo)
		if strings.EqualFold(session.DeviceID, deviceId) && strings.EqualFold(session.Client, client) {
			foundSession = session
			return false // Stop the iteration
		}
		return true // Continue the iteration
	})
	return foundSession
}

func (m *SessionManager) Sessions() []*session.SessionInfo {
	var sessions []*session.SessionInfo
	m.activeConnections.Range(func(key, value interface{}) bool {
		session := value.(*session.SessionInfo)
		sessions = append(sessions, session)
		return true
	})

	// Sort the sessions by LastActivityDate in descending order
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActivityDate.After(sessions[j].LastActivityDate)
	})

	return sessions
}

func (m *SessionManager) ReportTranscodingInfo(deviceId string, info *ms.TranscodingInfo) {
	var session *session.SessionInfo
	for _, value := range m.Sessions() {
		if strings.EqualFold(value.DeviceID, deviceId) {
			session = value
			break
		}
	}

	if session != nil {
		session.TranscodingInfo = *info
	}
}

func (m *SessionManager) ClearTranscodingInfo(deviceId string) {
	m.ReportTranscodingInfo(deviceId, nil)
}

func (m *SessionManager) Dispose() {
	// Cleanup resources here
}
