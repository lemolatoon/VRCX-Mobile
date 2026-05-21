// Package feed implements feed entry diffing and persistence.
// The diff logic is a Go port of VRCX's userEventCoordinator.js and friendPresenceCoordinator.js.
package feed

import "time"

// --- Entry types ---

type GPSEntry struct {
	ViewerUserID     string
	VRChatUserID     string
	DisplayName      string
	Location         string
	PreviousLocation string
	WorldName        *string
	GroupName        *string
	TimeSincePrevMs  *int64
	CreatedAt        time.Time
}

type StatusEntry struct {
	ViewerUserID               string
	VRChatUserID               string
	DisplayName                string
	Status                     string
	PreviousStatus             string
	StatusDescription          string
	PreviousStatusDescription  string
	CreatedAt                  time.Time
}

type BioEntry struct {
	ViewerUserID string
	VRChatUserID string
	DisplayName  string
	Bio          string
	PreviousBio  string
	CreatedAt    time.Time
}

type AvatarEntry struct {
	ViewerUserID                                string
	VRChatUserID                                string
	DisplayName                                 string
	OwnerID                                     string
	AvatarName                                  string
	CurrentAvatarImageURL                       string
	CurrentAvatarThumbnailImageURL              string
	PreviousCurrentAvatarImageURL               string
	PreviousCurrentAvatarThumbnailImageURL      string
	CreatedAt                                   time.Time
}

type OnlineOfflineEntry struct {
	ViewerUserID string
	VRChatUserID string
	DisplayName  string
	Type         string // "Online" | "Offline"
	Location     string
	WorldName    *string
	GroupName    *string
	TimeSinceMs  *int64
	CreatedAt    time.Time
}

// --- Diff ---

// Diff computes feed entries from a user snapshot transition.
// prevSnap / nextSnap are raw VRChat API user objects (unmarshaled as map[string]any).
// prevState / nextState are the presence state: "online" | "active" | "offline".
// Mirrors userEventCoordinator.js:90-338 and friendPresenceCoordinator.js:55-114.
//
// worldName / groupName resolution is deferred to the read API / client side;
// the corresponding entry fields are left nil here.
func Diff(
	viewerUserID string,
	prevSnap map[string]any,
	prevState string,
	nextSnap map[string]any,
	nextState string,
	now time.Time,
) []any {
	var entries []any

	vrcID := strField(nextSnap, "id")
	if vrcID == "" {
		vrcID = strField(prevSnap, "id")
	}
	displayName := strField(nextSnap, "displayName")
	if displayName == "" {
		displayName = strField(prevSnap, "displayName")
	}

	// --- Online / Offline (state transition; mirrors friendPresenceCoordinator.js:55-114) ---
	if prevState != nextState {
		prevOnline := prevState == "online"
		nextOnline := nextState == "online"
		if prevOnline && !nextOnline {
			// online → offline | active
			loc := strField(prevSnap, "location")
			entries = append(entries, OnlineOfflineEntry{
				ViewerUserID: viewerUserID,
				VRChatUserID: vrcID,
				DisplayName:  displayName,
				Type:         "Offline",
				Location:     loc,
				CreatedAt:    now,
			})
		} else if !prevOnline && nextOnline {
			// offline | active → online
			loc := strField(nextSnap, "location")
			entries = append(entries, OnlineOfflineEntry{
				ViewerUserID: viewerUserID,
				VRChatUserID: vrcID,
				DisplayName:  displayName,
				Type:         "Online",
				Location:     loc,
				CreatedAt:    now,
			})
		}
	}

	// --- Prop diffs (mirrors userEventCoordinator.js:90-338) ---

	// GPS: location changed, not traveling and not offline transition
	prevLoc := strField(prevSnap, "location")
	nextLoc := strField(nextSnap, "location")
	if prevLoc != nextLoc &&
		nextLoc != "traveling" &&
		nextLoc != "offline" && nextLoc != "" &&
		prevLoc != "offline" && prevLoc != "" &&
		prevState == nextState { // state change already emits Online/Offline above
		// traveling special case: use prevSnap's stored previousLocation if available
		// (we don't track $previousLocation in snapshot; simplified: use prevLoc as-is)
		if prevLoc != "" && prevLoc != "traveling" {
			entries = append(entries, GPSEntry{
				ViewerUserID:     viewerUserID,
				VRChatUserID:     vrcID,
				DisplayName:      displayName,
				Location:         nextLoc,
				PreviousLocation: prevLoc,
				CreatedAt:        now,
			})
		}
	}

	// Avatar: currentAvatarImageUrl or thumbnail changed
	prevImg := strField(prevSnap, "currentAvatarImageUrl")
	nextImg := strField(nextSnap, "currentAvatarImageUrl")
	prevThumb := strField(prevSnap, "currentAvatarThumbnailImageUrl")
	nextThumb := strField(nextSnap, "currentAvatarThumbnailImageUrl")
	if (prevImg != nextImg || prevThumb != nextThumb) &&
		nextImg != "" && prevImg != "" &&
		nextThumb != prevThumb { // if thumbnail matches, VRCX skips (imageMatches check)
		entries = append(entries, AvatarEntry{
			ViewerUserID:                           viewerUserID,
			VRChatUserID:                           vrcID,
			DisplayName:                            displayName,
			CurrentAvatarImageURL:                  nextImg,
			CurrentAvatarThumbnailImageURL:         nextThumb,
			PreviousCurrentAvatarImageURL:          prevImg,
			PreviousCurrentAvatarThumbnailImageURL: prevThumb,
			CreatedAt:                              now,
		})
	}

	// Status / statusDescription: changed and neither side is "offline"
	prevStatus := strField(prevSnap, "status")
	nextStatus := strField(nextSnap, "status")
	prevStatusDesc := strField(prevSnap, "statusDescription")
	nextStatusDesc := strField(nextSnap, "statusDescription")
	statusChanged := prevStatus != nextStatus || prevStatusDesc != nextStatusDesc
	neitherOffline := nextStatus != "offline" && prevStatus != "offline"
	if statusChanged && neitherOffline {
		entries = append(entries, StatusEntry{
			ViewerUserID:              viewerUserID,
			VRChatUserID:              vrcID,
			DisplayName:               displayName,
			Status:                    nextStatus,
			PreviousStatus:            prevStatus,
			StatusDescription:         nextStatusDesc,
			PreviousStatusDescription: prevStatusDesc,
			CreatedAt:                 now,
		})
	}

	// Bio: changed and non-empty on both sides
	prevBio := strField(prevSnap, "bio")
	nextBio := strField(nextSnap, "bio")
	if prevBio != nextBio && prevBio != "" && nextBio != "" {
		entries = append(entries, BioEntry{
			ViewerUserID: viewerUserID,
			VRChatUserID: vrcID,
			DisplayName:  displayName,
			Bio:          nextBio,
			PreviousBio:  prevBio,
			CreatedAt:    now,
		})
	}

	return entries
}

func strField(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	s, _ := v.(string)
	return s
}
