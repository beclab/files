package session

import (
	"files/pkg/media/mediabrowser/model/session"
)

type ISessionManager interface {
	/*
	   PlaybackStart() (EventHandler[PlaybackProgressEventArgs], error)
	   PlaybackProgress() (EventHandler[PlaybackProgressEventArgs], error)
	   PlaybackStopped() (EventHandler[PlaybackStopEventArgs], error)
	   SessionStarted() (EventHandler[SessionEventArgs], error)
	   SessionEnded() (EventHandler[SessionEventArgs], error)
	   SessionActivity() (EventHandler[SessionEventArgs], error)
	   SessionControllerConnected() (EventHandler[SessionEventArgs], error)
	   CapabilitiesChanged() (EventHandler[SessionEventArgs], error)
	   Sessions() ([]SessionInfo, error)
	   LogSessionActivity(appName, appVersion, deviceId, deviceName, remoteEndPoint string, user User) (*SessionInfo, error)
	   OnSessionControllerConnected(session *SessionInfo)
	   UpdateDeviceName(sessionId, reportedDeviceName string)
	   OnPlaybackStart(info *PlaybackStartInfo) error
	   OnPlaybackProgress(info *PlaybackProgressInfo) error
	   OnPlaybackProgress2(info *PlaybackProgressInfo, isAutomated bool) error
	   OnPlaybackStopped(info *PlaybackStopInfo) error
	   ReportSessionEnded(sessionId string) (ValueTask, error)
	   SendGeneralCommand(controllingSessionId, sessionId string, command GeneralCommand, cancellationToken context.CancellationToken) error
	   SendMessageCommand(controllingSessionId, sessionId string, command MessageCommand, cancellationToken context.CancellationToken) error
	   SendPlayCommand(controllingSessionId, sessionId string, command PlayRequest, cancellationToken context.CancellationToken) error
	   SendSyncPlayCommand(sessionId string, command SendCommand, cancellationToken context.CancellationToken) error
	   SendSyncPlayGroupUpdate[T](sessionId string, command GroupUpdate[T], cancellationToken context.CancellationToken) error
	   SendBrowseCommand(controllingSessionId, sessionId string, command BrowseRequest, cancellationToken context.CancellationToken) error
	   SendPlaystateCommand(controllingSessionId, sessionId string, command PlaystateRequest, cancellationToken context.CancellationToken) error
	   SendMessageToAdminSessions[T](messageType SessionMessageType, data T, cancellationToken context.CancellationToken) error
	   SendMessageToUserSessions[T](userIds []uuid.UUID, messageType SessionMessageType, data T, cancellationToken context.CancellationToken) error
	   SendMessageToUserSessions[T](userIds []uuid.UUID, messageType SessionMessageType, dataFn func() T, cancellationToken context.CancellationToken) error
	   SendMessageToUserDeviceSessions[T](deviceId string, messageType SessionMessageType, data T, cancellationToken context.CancellationToken) error
	   SendRestartRequiredNotification(cancellationToken context.CancellationToken) error
	   AddAdditionalUser(sessionId string, userId uuid.UUID)
	   RemoveAdditionalUser(sessionId string, userId uuid.UUID)
	   ReportNowViewingItem(sessionId, itemId string)
	   AuthenticateNewSession(request *AuthenticationRequest) (*AuthenticationResult, error)
	   AuthenticateDirect(request *AuthenticationRequest) (*AuthenticationResult, error)
	   ReportCapabilities(sessionId string, capabilities *ClientCapabilities)
	*/
	ReportTranscodingInfo(deviceId string, info *session.TranscodingInfo)
	ClearTranscodingInfo(deviceId string)
	GetSession(deviceId, client, version string) *SessionInfo
	/*
	   GetSessionByAuthenticationToken(token, deviceId, remoteEndpoint string) (*SessionInfo, error)
	   GetSessionByAuthenticationToken2(device *Device, deviceId, remoteEndpoint, appVersion string) (*SessionInfo, error)
	   Logout(accessToken string) error
	   Logout2(device *Device) error
	   RevokeUserTokens(userId uuid.UUID, currentAccessToken string) error
	   CloseIfNeededAsync(session *SessionInfo) error
	*/
}
