// Package parser converts VRChat output_log lines into mobile GameLog entries.
package parser

import (
	"regexp"
	"strings"
	"time"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/gamelog"
)

var (
	cleanID       = regexp.MustCompile(`[^a-zA-Z0-9_\-~:()]`)
	cleanLocation = regexp.MustCompile(`/`)
	logPrefix     = regexp.MustCompile(`^(\d{4}\.\d{2}\.\d{2} \d{2}:\d{2}:\d{2})\s+\S+\s+-\s+(.*)$`)
)

type FileContext struct {
	RecentWorldName     string
	LocationDestination string
	LastAudioDevice     string
	AudioDeviceChanged  bool
	ShaderLimitSeen     bool
	VideoErrors         map[string]bool
}

func NewFileContext() *FileContext {
	return &FileContext{VideoErrors: map[string]bool{}}
}

func ParseLine(logFile string, offset int64, line string, ctx *FileContext) (*gamelog.Entry, bool) {
	if ctx == nil {
		ctx = NewFileContext()
	}
	line = strings.TrimRight(line, "\r\n")
	if strings.TrimSpace(line) == "" {
		return nil, false
	}
	if strings.Contains(line, "[PyPyDance]") {
		return entry(logFile, offset, time.Now().UTC(), "Event", map[string]any{"eventType": "UdonException", "message": line}, line), true
	}
	if strings.Contains(line, " ---> VRC.Udon.VM.UdonVMException: ") {
		return entry(logFile, offset, time.Now().UTC(), "Event", map[string]any{
			"eventType": "UdonException",
			"message":   line[strings.Index(line, " ---> VRC.Udon.VM.UdonVMException: "):],
		}, line), true
	}
	createdAt, body, ok := splitLine(line)
	if !ok {
		return entry(logFile, offset, time.Now().UTC(), "Unknown", map[string]any{"message": line}, line), true
	}

	if strings.Contains(body, "[Behaviour] Entering Room: ") {
		ctx.RecentWorldName = afterLast(body, "] Entering Room: ")
		return nil, false
	}
	if strings.Contains(body, "[Behaviour] Joining ") &&
		!strings.Contains(body, "] Joining or Creating Room: ") &&
		!strings.Contains(body, "] Joining friend: ") {
		location := cleanLocation.ReplaceAllString(afterLast(body, "] Joining "), "")
		ctx.LastAudioDevice = ""
		ctx.VideoErrors = map[string]bool{}
		return entry(logFile, offset, createdAt, "Location", map[string]any{
			"location": location, "worldName": ctx.RecentWorldName,
		}, line), true
	}
	if strings.Contains(body, "[Behaviour] Destination fetching: ") {
		ctx.LocationDestination = cleanLocation.ReplaceAllString(afterLast(body, "] Destination fetching: "), "")
		return nil, false
	}
	if strings.Contains(body, "[Behaviour] OnLeftRoom") || strings.Contains(body, "[Behaviour] Successfully left room") {
		location := ctx.LocationDestination
		ctx.LocationDestination = ""
		return entry(logFile, offset, createdAt, "LocationDestination", map[string]any{"location": location}, line), true
	}
	if strings.Contains(body, "[Behaviour] OnPlayerJoined") && !strings.Contains(body, "] OnPlayerJoined:") {
		displayName, userID := parseUserInfo(afterLast(body, "] OnPlayerJoined"))
		return entry(logFile, offset, createdAt, "OnPlayerJoined", map[string]any{
			"displayName": displayName, "userId": userID,
		}, line), true
	}
	if strings.Contains(body, "[Behaviour] OnPlayerLeft") &&
		!strings.Contains(body, "] OnPlayerLeftRoom") &&
		!strings.Contains(body, "] OnPlayerLeft:") {
		displayName, userID := parseUserInfo(afterLast(body, "] OnPlayerLeft"))
		return entry(logFile, offset, createdAt, "OnPlayerLeft", map[string]any{
			"displayName": displayName, "userId": userID,
		}, line), true
	}
	if strings.Contains(body, "[Behaviour] Instantiated a (Clone [") && strings.Contains(body, "] Portals/PortalInternalDynamic)") {
		return entry(logFile, offset, createdAt, "PortalSpawn", map[string]any{}, line), true
	}
	if strings.HasPrefix(body, "[Network Processing] RPC invoked SwitchAvatar on AvatarPedestal for ") {
		displayName := strings.TrimPrefix(body, "[Network Processing] RPC invoked SwitchAvatar on AvatarPedestal for ")
		return eventEntry(logFile, offset, createdAt, line, displayName+" changed avatar pedestal")
	}
	if strings.HasPrefix(body, "[Video Playback] Attempting to resolve URL '") {
		return videoEntry(logFile, offset, createdAt, line, strings.TrimSuffix(strings.TrimPrefix(body, "[Video Playback] Attempting to resolve URL '"), "'"), "")
	}
	if strings.HasPrefix(body, "[Video Playback] Resolving URL '") {
		return videoEntry(logFile, offset, createdAt, line, strings.TrimSuffix(strings.TrimPrefix(body, "[Video Playback] Resolving URL '"), "'"), "")
	}
	if strings.HasPrefix(body, "User ") && strings.Contains(body, " added URL ") {
		pos := strings.LastIndex(body, " added URL ")
		return videoEntry(logFile, offset, createdAt, line, body[pos+11:], body[5:pos])
	}
	if strings.HasPrefix(body, "[USharpVideo] Started video load for URL: ") && strings.Contains(body, ", requested by ") {
		raw := strings.TrimPrefix(body, "[USharpVideo] Started video load for URL: ")
		pos := strings.LastIndex(raw, ", requested by ")
		return videoEntry(logFile, offset, createdAt, line, raw[:pos], raw[pos+15:])
	}
	if strings.HasPrefix(body, "[USharpVideo] Syncing video to ") {
		return eventEntry(logFile, offset, createdAt, line, "Video sync: "+strings.TrimPrefix(body, "[USharpVideo] Syncing video to "))
	}
	if strings.HasPrefix(body, "[API] Received Notification: <") && strings.Contains(body, "> received at ") {
		raw := strings.TrimPrefix(body, "[API] Received Notification: <")
		raw = raw[:strings.LastIndex(raw, "> received at ")]
		return entry(logFile, offset, createdAt, "Event", map[string]any{"eventType": "Notification", "message": raw}, line), true
	}
	if strings.HasPrefix(body, "[API] [") && strings.Contains(body, "] Sending Get request to ") {
		return entry(logFile, offset, createdAt, "Event", map[string]any{
			"eventType": "APIRequest",
			"message":   afterLast(body, "] Sending Get request to "),
		}, line), true
	}
	if strings.HasPrefix(body, "[Behaviour] Switching ") && strings.Contains(body, " to avatar ") {
		raw := strings.TrimPrefix(body, "[Behaviour] Switching ")
		pos := strings.LastIndex(raw, " to avatar ")
		return entry(logFile, offset, createdAt, "Event", map[string]any{
			"eventType":   "AvatarChange",
			"displayName": raw[:pos],
			"avatarName":  raw[pos+11:],
			"message":     raw[:pos] + " switched to avatar " + raw[pos+11:],
		}, line), true
	}
	if strings.Contains(body, "] Attempting to load String from URL '") {
		url := strings.TrimSuffix(afterLast(body, "] Attempting to load String from URL '"), "'")
		if isOwnURL(url) {
			return nil, false
		}
		return entry(logFile, offset, createdAt, "ResourceLoad", map[string]any{"resourceType": "StringLoad", "resourceUrl": url}, line), true
	}
	if strings.Contains(body, "] Attempting to load image from URL '") {
		url := strings.TrimSuffix(afterLast(body, "] Attempting to load image from URL '"), "'")
		if isOwnURL(url) {
			return nil, false
		}
		return entry(logFile, offset, createdAt, "ResourceLoad", map[string]any{"resourceType": "ImageLoad", "resourceUrl": url}, line), true
	}
	if strings.Contains(body, "[Video Playback] ERROR: ") {
		return dedupEvent(ctx, logFile, offset, createdAt, line, "VideoError: "+afterLast(body, "[Video Playback] ERROR: "))
	}
	if strings.Contains(body, "[AVProVideo] Error: ") {
		return dedupEvent(ctx, logFile, offset, createdAt, line, "VideoError: "+afterLast(body, "[AVProVideo] Error: "))
	}
	if strings.Contains(body, "Attempted to play an untrusted URL") {
		return dedupEvent(ctx, logFile, offset, createdAt, line, "VideoError: "+body)
	}
	if strings.HasPrefix(body, "[VRCX] ") {
		return eventEntry(logFile, offset, createdAt, line, strings.TrimPrefix(body, "[VRCX] "))
	}
	if strings.HasPrefix(body, "[VRCX-World] ") {
		return nil, false
	}
	if strings.Contains(body, "[VRC Camera] Took screenshot to: ") {
		return entry(logFile, offset, createdAt, "Event", map[string]any{
			"eventType": "Screenshot", "message": afterLast(body, "] Took screenshot to: "),
		}, line), true
	}
	if strings.Contains(body, "[Always] uSpeak: OnAudioConfigurationChanged") {
		ctx.AudioDeviceChanged = true
		return nil, false
	}
	if strings.Contains(body, "[Always] uSpeak: SetInputDevice 0") {
		audioDevice := betweenLast(body, ") '", "'")
		if audioDevice == "" {
			return nil, false
		}
		if ctx.LastAudioDevice == "" {
			ctx.AudioDeviceChanged = false
			ctx.LastAudioDevice = audioDevice
			return nil, false
		}
		if !ctx.AudioDeviceChanged || ctx.LastAudioDevice == audioDevice {
			return nil, false
		}
		ctx.LastAudioDevice = audioDevice
		ctx.AudioDeviceChanged = false
		return eventEntry(logFile, offset, createdAt, line, "Audio device changed, mic set to '"+audioDevice+"'")
	}
	if strings.Contains(body, "Maximum number (384) of shader global keywords exceeded") {
		if ctx.ShaderLimitSeen {
			return nil, false
		}
		ctx.ShaderLimitSeen = true
		return eventEntry(logFile, offset, createdAt, line, "Shader Keyword Limit has been reached")
	}
	if strings.Contains(body, "] Master is not sending any events! Moving to a new instance.") {
		return eventEntry(logFile, offset, createdAt, line, "Joining instance blocked by master")
	}
	if strings.Contains(body, "[Behaviour] Received executive message: ") {
		return eventEntry(logFile, offset, createdAt, line, afterLast(body, "[Behaviour] Received executive message: "))
	}
	if strings.Contains(body, "[Behaviour] Failed to join instance ") {
		return eventEntry(logFile, offset, createdAt, line, strings.TrimPrefix(body, "[Behaviour] Failed to join instance "))
	}
	if strings.HasPrefix(body, "Could not Start OSC: ") {
		return eventEntry(logFile, offset, createdAt, line, "VRChat couldn't start OSC server, \""+body+"\"")
	}
	if strings.Contains(body, "[ModerationManager] This instance will be reset in ") {
		return eventEntry(logFile, offset, createdAt, line, afterLast(body, "[ModerationManager] "))
	}
	if strings.Contains(body, "[ModerationManager] A vote kick has been initiated against ") {
		return eventEntry(logFile, offset, createdAt, line, afterLast(body, "[ModerationManager] "))
	}
	if strings.Contains(body, "[ModerationManager] Vote to kick ") {
		return eventEntry(logFile, offset, createdAt, line, afterLast(body, "[ModerationManager] "))
	}
	if strings.Contains(body, "[StickersManager] User ") && strings.Contains(body, "spawned sticker") && strings.Contains(body, "inv_") {
		info := afterLast(body, "[StickersManager] User ")
		first, second := parseUserInfo(info)
		userID, displayName := first, second
		invIndex := strings.Index(info, "inv_")
		inventoryID := ""
		if invIndex >= 0 {
			inventoryID = cleanID.ReplaceAllString(info[invIndex:], "")
		}
		return entry(logFile, offset, createdAt, "Event", map[string]any{
			"eventType":   "StickerSpawn",
			"userId":      userID,
			"displayName": displayName,
			"inventoryId": inventoryID,
			"message":     displayName + " spawned sticker " + inventoryID,
		}, line), true
	}
	if strings.HasPrefix(body, "VRCApplication: OnApplicationQuit at ") || strings.HasPrefix(body, "VRCApplication: HandleApplicationQuit at ") {
		return entry(logFile, offset, createdAt, "Event", map[string]any{"eventType": "VRCQuit", "message": "VRChat quit"}, line), true
	}
	if strings.HasPrefix(body, "Initializing VRSDK.") || strings.HasPrefix(body, "STEAMVR HMD Model: ") {
		return entry(logFile, offset, createdAt, "Event", map[string]any{"eventType": "OpenVRInit", "message": body}, line), true
	}
	if strings.HasPrefix(body, "VR Disabled") {
		return entry(logFile, offset, createdAt, "Event", map[string]any{"eventType": "DesktopMode", "message": body}, line), true
	}
	return entry(logFile, offset, createdAt, "Unknown", map[string]any{"message": body}, line), true
}

func splitLine(line string) (time.Time, string, bool) {
	m := logPrefix.FindStringSubmatch(line)
	if len(m) != 3 {
		return time.Time{}, "", false
	}
	t, err := time.ParseInLocation("2006.01.02 15:04:05", m[1], time.Local)
	if err != nil {
		return time.Time{}, "", false
	}
	return t.UTC(), strings.TrimSpace(m[2]), true
}

func entry(logFile string, offset int64, createdAt time.Time, typ string, payload map[string]any, raw string) *gamelog.Entry {
	return &gamelog.Entry{LogFile: logFile, LineOffset: offset, CreatedAt: createdAt, Type: typ, Payload: payload, RawLine: raw}
}

func videoEntry(logFile string, offset int64, createdAt time.Time, raw, url, displayName string) (*gamelog.Entry, bool) {
	return entry(logFile, offset, createdAt, "VideoPlay", map[string]any{"videoUrl": url, "displayName": displayName}, raw), true
}

func eventEntry(logFile string, offset int64, createdAt time.Time, raw, message string) (*gamelog.Entry, bool) {
	return entry(logFile, offset, createdAt, "Event", map[string]any{"message": message}, raw), true
}

func dedupEvent(ctx *FileContext, logFile string, offset int64, createdAt time.Time, raw, message string) (*gamelog.Entry, bool) {
	if ctx.VideoErrors[message] {
		return nil, false
	}
	ctx.VideoErrors[message] = true
	return eventEntry(logFile, offset, createdAt, raw, message)
}

func afterLast(s, marker string) string {
	pos := strings.LastIndex(s, marker)
	if pos < 0 {
		return ""
	}
	return strings.TrimSpace(s[pos+len(marker):])
}

func betweenLast(s, start, end string) string {
	pos := strings.LastIndex(s, start)
	if pos < 0 {
		return ""
	}
	pos += len(start)
	rest := s[pos:]
	endPos := strings.Index(rest, end)
	if endPos < 0 {
		return ""
	}
	return rest[:endPos]
}

func parseUserInfo(s string) (string, string) {
	s = strings.TrimSpace(s)
	pos := strings.LastIndex(s, " (")
	if pos < 0 {
		return s, ""
	}
	end := strings.LastIndex(s, ")")
	if end < pos {
		return s, ""
	}
	return s[:pos], cleanID.ReplaceAllString(s[pos+2:end], "")
}

func isOwnURL(url string) bool {
	return strings.HasPrefix(url, "http://127.0.0.1:22500") || strings.HasPrefix(url, "http://localhost:22500")
}
