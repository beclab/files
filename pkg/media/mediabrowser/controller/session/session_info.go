package session

import (
	"time"

	"files/pkg/media/mediabrowser/model/session"
)

type SessionInfo struct {
	ProgressIncrement int64
	sessionManager    ISessionManager
	//    logger                ILogger
	progressLock  interface{}
	progressTimer *time.Timer
	//    lastProgressInfo      PlaybackProgressInfo
	disposed bool
	//    PlayState             PlayerStateInfo
	//    AdditionalUsers       []SessionUserInfo
	//    Capabilities          ClientCapabilities
	RemoteEndPoint string
	//    PlayableMediaTypes    []MediaType
	ID                  string
	UserID              string
	UserName            string
	Client              string
	LastActivityDate    time.Time
	LastPlaybackCheckIn time.Time
	LastPausedDate      *time.Time
	DeviceName          string
	DeviceType          string
	//    NowPlayingItem        BaseItemDto
	//    FullNowPlayingItem    BaseItem
	//    NowViewingItem        BaseItemDto
	DeviceID           string
	ApplicationVersion string
	//    SessionControllers    []ISessionController
	TranscodingInfo session.TranscodingInfo
	//    NowPlayingQueue        []QueueItem
	//    NowPlayingQueueFullItems []BaseItemDto
	HasCustomDeviceName bool
	PlaylistItemId      string
	ServerId            string
	UserPrimaryImageTag string
	// Capabilities           *Capabilities
}

/*

type ISessionManager interface {
    // Define the methods for ISessionManager interface
}

type ILogger interface {
    // Define the methods for ILogger interface
}

type PlayerStateInfo struct {
    // Define the fields for PlayerStateInfo struct
}

type SessionUserInfo struct {
    // Define the fields for SessionUserInfo struct
}

type ClientCapabilities struct {
    PlayableMediaTypes []MediaType
    // Define other fields for ClientCapabilities struct
}

type MediaType int

type BaseItemDto struct {
    // Define the fields for BaseItemDto struct
}

type BaseItem struct {
    // Define the fields for BaseItem struct
}


type ISessionController interface {
    // Define the methods for ISessionController interface
}

func (s *SessionInfo) IsActive() bool {
    for _, controller := range s.SessionControllers {
        if controller.IsSessionActive() {
            return true
        }
    }

    if len(s.SessionControllers) > 0 {
        return false
    }

    return true
}

func (s *SessionInfo) SupportsMediaControl() bool {
    if s.Capabilities == nil || !s.Capabilities.SupportsMediaControl {
        return false
    }

    for _, controller := range s.SessionControllers {
        if controller.SupportsMediaControl() {
            return true
        }
    }

    return false
}


func (s *SessionInfo) SupportsRemoteControl() bool {
    if s.Capabilities == nil || !s.Capabilities.SupportsMediaControl {
        return false
    }

    for _, controller := range s.SessionControllers {
        if controller.SupportsMediaControl() {
            return true
        }
    }

    return false
}

func (s *SessionInfo) GetSupportedCommands() []GeneralCommandType {
    if s.Capabilities == nil {
        return []GeneralCommandType{}
    }
    return s.Capabilities.SupportedCommands
}

func (s *SessionInfo) EnsureController[T any](factory func(*SessionInfo) ISessionController) (ISessionController, bool) {
    controllers := make([]ISessionController, len(s.SessionControllers))
    copy(controllers, s.SessionControllers)

    for _, controller := range controllers {
        if _, ok := controller.(T); ok {
            return controller, false
        }
    }

    newController := factory(s)
    s.Logger.LogDebug("Creating new %T", newController)
    s.SessionControllers = append(s.SessionControllers, newController)

    return newController, true
}


func (s *SessionInfo) AddController(controller ISessionController) {
    s.SessionControllers = append(s.SessionControllers, controller)
}

func (s *SessionInfo) ContainsUser(userId uuid.UUID) bool {
    if s.UserId == userId {
        return true
    }

    for _, additionalUser := range s.AdditionalUsers {
        if additionalUser.UserId == userId {
            return true
        }
    }

    return false
}

func (s *SessionInfo) StartAutomaticProgress(progressInfo *PlaybackProgressInfo) {
    if s.disposed {
        return
    }

    s.progressLock.Lock()
    defer s.progressLock.Unlock()

    s.lastProgressInfo = progressInfo

    if s.progressTimer == nil {
        s.progressTimer = time.AfterFunc(1*time.Second, s.onProgressTimerCallback)
    } else {
        s.progressTimer.Reset(1 * time.Second)
    }
}

func (s *SessionInfo) onProgressTimerCallback() {
    if s.disposed {
        return
    }

    progressInfo := s.lastProgressInfo
    if progressInfo == nil {
        return
    }

    if progressInfo.IsPaused {
        return
    }

    positionTicks := progressInfo.PositionTicks
    if positionTicks < 0 {
        positionTicks = 0
    }

    newPositionTicks := positionTicks + s.progressIncrement
    item := progressInfo.Item
    runtimeTicks := int64(0)
    if item != nil {
        runtimeTicks = item.RunTimeTicks
    }

    // Don't report beyond the runtime
    if runtimeTicks > 0 && newPositionTicks >= runtimeTicks {
        return
    }

    progressInfo.PositionTicks = newPositionTicks

    err := s.sessionManager.OnPlaybackProgress(progressInfo, true)
    if err != nil {
        s.logger.Printf("Error reporting playback progress: %v", err)
    }
}

func (s *SessionInfo) stopAutomaticProgress() {
    s.progressLock.Lock()
    defer s.progressLock.Unlock()

    if s.progressTimer != nil {
        s.progressTimer.Stop()
        s.progressTimer = nil
    }

    s.lastProgressInfo = nil
}

func (s *SessionInfo) disposeAsync() error {
    s.disposed = true

    s.stopAutomaticProgress()

    controllers := make([]ISessionController, len(s.sessionControllers))
    copy(controllers, s.sessionControllers)
    s.sessionControllers = nil

    var wg sync.WaitGroup
    for _, controller := range controllers {
        wg.Add(1)
        go func(c ISessionController) {
            defer wg.Done()
            if asyncController, ok := c.(ISessionController); ok {
                s.logger.Printf("Disposing session controller asynchronously %T", asyncController)
                err := asyncController.DisposeAsync()
                if err != nil {
                    s.logger.Printf("Error disposing session controller asynchronously %T: %v", asyncController, err)
                }
            } else if syncController, ok := c.(IDisposable); ok {
                s.logger.Printf("Disposing session controller synchronously %T", syncController)
                syncController.Dispose()
            }
        }(controller)
    }
    wg.Wait()

    return nil
}
*/
