package helpers

import (
	"errors"
	// "strings"
)

type ItemSortBy int
type SortOrder int

const (
	SortOrder_Ascending SortOrder = iota
	SortOrder_Descending
)

type SortInfo struct {
	SortBy ItemSortBy
	Order  SortOrder
}

func GetOrderBy(sortBy []ItemSortBy, requestedSortOrder []SortOrder) []SortInfo {
	if len(sortBy) == 0 {
		return []SortInfo{}
	}

	result := make([]SortInfo, len(sortBy))
	i := 0
	// Add elements which have a SortOrder specified
	for ; i < len(requestedSortOrder); i++ {
		result[i] = SortInfo{
			SortBy: sortBy[i],
			Order:  requestedSortOrder[i],
		}
	}

	// Add remaining elements with the first specified SortOrder
	// or the default one if no SortOrders are specified
	order := SortOrder_Ascending
	if len(requestedSortOrder) > 0 {
		order = requestedSortOrder[0]
	}
	for ; i < len(sortBy); i++ {
		result[i] = SortInfo{
			SortBy: sortBy[i],
			Order:  order,
		}
	}

	return result
}

func GetUserId(claimsPrincipal any, userId *string) (string, error) {
	authenticatedUserId := getAuthenticatedUserId(claimsPrincipal)

	// UserId not provided, fall back to authenticated user id.
	if userId == nil || *userId == "" {
		return authenticatedUserId, nil
	}

	// User must be administrator to access another user.
	isAdministrator := isInRole(claimsPrincipal, "Administrator")
	if *userId != authenticatedUserId && !isAdministrator {
		return "", errors.New("Forbidden")
	}

	return *userId, nil
}

/*
func AssertCanUpdateUser(userManager any, claimsPrincipal any, userId string, restrictUserPreferences bool) bool {
    authenticatedUserId := getAuthenticatedUserId(claimsPrincipal)
    isAdministrator := isInRole(claimsPrincipal, "Administrator")

    // If they're going to update the record of another user, they must be an administrator
    if userId != authenticatedUserId && !isAdministrator {
        return false
    }

    // TODO the EnableUserPreferenceAccess policy does not seem to be used elsewhere
    if !restrictUserPreferences || isAdministrator {
        return true
    }

    user := getUserById(userManager, userId)
    if user == nil {
        return false
    }

    return user.EnableUserPreferenceAccess
}
*/

type SessionInfo struct {
	Id string
}

/*
func GetSession(sessionManager any, userManager any, httpContext any, userId *string) (*SessionInfo, error) {
    if userId == nil || *userId == "" {
        userId = getAuthenticatedUserId(httpContext)
    }

    user := getUserById(userManager, *userId)

    session := logSessionActivity(
        sessionManager,
        getClient(httpContext),
        getVersion(httpContext),
        getDeviceId(httpContext),
        getDevice(httpContext),
        getNormalizedRemoteIP(httpContext),
        user,
    )

    if session == nil {
        return nil, errors.New("Session not found")
    }

    return &SessionInfo{
        Id: session.Id,
    }, nil
}
*/

/*
func GetSessionId(sessionManager any, userManager any, httpContext any) (string, error) {
    session, err := GetSession(sessionManager, userManager, httpContext, nil)
    if err != nil {
        return "", err
    }

    return session.Id, nil
}
*/

type BaseItemDto struct {
	ChildCount   int
	ProgramCount int
	SeriesCount  int
	EpisodeCount int
	MovieCount   int
	TrailerCount int
	AlbumCount   int
	SongCount    int
	ArtistCount  int
}

type QueryResult struct {
	StartIndex       int
	TotalRecordCount int
	Items            []BaseItemDto
}

/*
func CreateQueryResult(
    result []Tuple[any, any],
    dtoOptions any,
    dtoService any,
    includeItemTypes bool,
    user any,
) *QueryResult {
    var dtos []BaseItemDto
    for _, i := range result {
        baseItem, counts := i[0], i[1]
        dto := getDtoByNameDto(dtoService, baseItem, dtoOptions, nil, user)

        if includeItemTypes {
            dto.ChildCount = counts.ItemCount
            dto.ProgramCount = counts.ProgramCount
            dto.SeriesCount = counts.SeriesCount
            dto.EpisodeCount = counts.EpisodeCount
            dto.MovieCount = counts.MovieCount
            dto.TrailerCount = counts.TrailerCount
            dto.AlbumCount = counts.AlbumCount
            dto.SongCount = counts.SongCount
            dto.ArtistCount = counts.ArtistCount
        }

        dtos = append(dtos, dto)
    }

    return &QueryResult{
        StartIndex:       0,
        TotalRecordCount: len(dtos),
        Items:            dtos,
    }
}

type Tuple[T1, T2] struct {
    Item1 T1
    Item2 T2
}
*/

func getAuthenticatedUserId(claimsPrincipal any) string {
	// implementation depends on the authentication mechanism
	return ""
}

func isInRole(claimsPrincipal any, role string) bool {
	// implementation depends on the authorization mechanism
	return false
}

func getUserById(userManager any, userId string) any {
	// implementation depends on the user management mechanism
	return nil
}

func getClient(httpContext any) string {
	// implementation depends on the HTTP context
	return ""
}

func getVersion(httpContext any) string {
	// implementation depends on the HTTP context
	return ""
}

func getDeviceId(httpContext any) string {
	// implementation depends on the HTTP context
	return ""
}

func getDevice(httpContext any) string {
	// implementation depends on the HTTP context
	return ""
}

func getNormalizedRemoteIP(httpContext any) string {
	// implementation depends on the HTTP context
	return ""
}

func logSessionActivity(
	sessionManager any,
	client string,
	version string,
	deviceId string,
	device string,
	remoteIP string,
	user any,
) any {
	// implementation depends on the session management mechanism
	return nil
}

func getDtoByNameDto(
	dtoService any,
	baseItem any,
	dtoOptions any,
	_ any,
	user any,
) BaseItemDto {
	// implementation depends on the DTO service
	return BaseItemDto{}
}
