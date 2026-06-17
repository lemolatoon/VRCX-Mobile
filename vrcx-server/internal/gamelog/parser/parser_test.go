package parser

import (
	"strings"
	"testing"
)

func TestParseLineVRCXLogWatcherCommentExamples(t *testing.T) {
	ctx := NewFileContext()

	// Keep this list aligned with the concrete log examples documented in
	// Dotnet/LogWatcher.cs. expected=false means VRCX consumes/ignores the line
	// without appending a GameLog row; Unknown means the mobile agent preserves a
	// non-consumed line for forward compatibility.
	cases := []struct {
		name     string
		line     string
		expected string
		key      string
		want     string
	}{
		{"old destination fetching", "2020.10.31 23:36:28 Log        -  [VRCFlowManagerVRC] Destination fetching: wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd", "Unknown", "", ""},
		{"old destination set", "2020.10.31 23:36:28 Log        -  [VRCFlowManagerVRC] Destination set: wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd", "Unknown", "", ""},
		{"roommanager entering room", "2020.10.31 23:36:31 Log        -  [RoomManager] Entering Room: VRChat Home", "Unknown", "", ""},
		{"roommanager joining", "2020.10.31 23:36:31 Log        -  [RoomManager] Joining wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd:67646~private(usr_4f76a584-9d4b-46f6-8209-8305eb683661)~nonce(D9298A536FEEEDDBB61633661A4BDAA09717C5178DEF865C4C09372FE12E09A6)", "Unknown", "", ""},
		{"roommanager joining or creating", "2020.10.31 23:36:31 Log        -  [RoomManager] Joining or Creating Room: VRChat Home", "Unknown", "", ""},
		{"roommanager successfully joined", "2020.10.31 23:36:31 Log        -  [RoomManager] Successfully joined room", "Unknown", "", ""},
		{"obfuscated destination fetching", "2021.02.03 10:18:58 Log        -  [ǄǄǅǅǅǄǄǅǅǄǅǅǅǅǄǄǄǅǅǄǄǅǅǅǅǄǅǅǅǅǄǄǄǄǄǅǄǅǄǄǄǅǅǄǅǅǅ] Destination fetching: wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd", "Unknown", "", ""},
		{"behaviour entering room", "2021.06.23 12:02:56 Log        -  [Behaviour] Entering Room: VRChat Home", "", "", ""},
		{"behaviour joining", "2021.06.23 12:02:57 Log        -  [Behaviour] Joining wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd:67646~private(usr_4f76a584-9d4b-46f6-8209-8305eb683661)~nonce(D9298A536FEEEDDBB61633661A4BDAA09717C5178DEF865C4C09372FE12E09A6)", "Location", "worldName", "VRChat Home"},
		{"screenshot", `2023.02.08 12:31:35 Log        -  [VRC Camera] Took screenshot to: C:\Users\Tea\Pictures\VRChat\2023-02\VRChat_2023-02-08_12-31-35.104_1920x1080.png`, "Event", "eventType", "Screenshot"},
		{"destination set", "2021.09.02 00:02:12 Log        -  [Behaviour] Destination set: wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd:15609~private(usr_032383a7-748c-4fb2-94e4-bcb928e5de6b)~nonce(72CC87D420C1D49AEFFBEE8824C84B2DF0E38678E840661E)", "Unknown", "", ""},
		{"destination fetching", "2021.09.02 00:49:15 Log        -  [Behaviour] Destination fetching: wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd", "", "", ""},
		{"on left room", "2022.08.13 18:57:00 Log        -  [Behaviour] OnLeftRoom", "LocationDestination", "location", "wrld_4432ea9b-729c-46e3-8eaf-846aa0a37fdd"},
		{"successfully left room", "2024.11.22 15:32:28 Log        -  [Behaviour] Successfully left room", "LocationDestination", "location", ""},
		{"debug left room", "2025.11.20 01:35:38 Debug      -  [Behaviour] OnLeftRoom", "LocationDestination", "location", ""},
		{"old network joined", "2020.10.31 23:36:58 Log        -  [NetworkManager] OnPlayerJoined pypy", "Unknown", "", ""},
		{"player initialized local", `2020.10.31 23:36:58 Log        -  [Player] Initialized PlayerAPI "pypy" is local`, "Unknown", "", ""},
		{"old network joined unicode", "2020.10.31 23:36:58 Log        -  [NetworkManager] OnPlayerJoined Rize♡", "Unknown", "", ""},
		{"player initialized remote", `2020.10.31 23:36:58 Log        -  [Player] Initialized PlayerAPI "Rize♡" is remote`, "Unknown", "", ""},
		{"old network left", "2020.11.01 00:07:01 Log        -  [NetworkManager] OnPlayerLeft Rize♡", "Unknown", "", ""},
		{"player manager removed", "2020.11.01 00:07:01 Log        -  [PlayerManager] Removed player 2 / Rize♡", "Unknown", "", ""},
		{"player unregistering", "2020.11.01 00:07:02 Log        -  [Player] Unregistering Rize♡", "Unknown", "", ""},
		{"initialized player api", `2021.06.23 11:41:16 Log        -  [Behaviour] Initialized PlayerAPI "Natsumi-sama" is local`, "Unknown", "", ""},
		{"joined without id", "2021.12.12 11:47:22 Log        -  [Behaviour] OnPlayerJoined Natsumi-sama", "OnPlayerJoined", "displayName", "Natsumi-sama"},
		{"joined colon ignored", "2021.12.12 11:47:22 Log        -  [Behaviour] OnPlayerJoined:Unnamed", "Unknown", "", ""},
		{"left room ignored by join parser", "2021.12.12 11:53:14 Log        -  [Behaviour] OnPlayerLeftRoom", "Unknown", "", ""},
		{"old portal rpc", "2021.04.06 11:25:45 Log        -  [Network Processing] RPC invoked ConfigurePortal on (Clone [1600004] Portals/PortalInternalDynamic) for Natsumi-sama", "Unknown", "", ""},
		{"old portal sendrpc", `2021.07.19 04:24:28 Log        -  [Behaviour] Will execute SendRPC/AlwaysBufferOne on (Clone [100004] Portals/PortalInternalDynamic) (UnityEngine.GameObject) for Natsumi-sama: S: "ConfigurePortal" I: 7 F: 0 B: 255 (local master owner)`, "Unknown", "", ""},
		{"portal spawn", "2022.07.29 18:40:37 Log        -  [Behaviour] Instantiated a (Clone [800004] Portals/PortalInternalDynamic)", "PortalSpawn", "", ""},
		{"shader 256 ignored by vrcx", "2021.04.04 12:21:06 Error      -  Maximum number (256) of shader keywords exceeded, keyword _TOGGLESIMPLEBLUR_ON will be ignored.", "Unknown", "", ""},
		{"shader 384 invalid timestamp", "2021.08.20 04:20:69 Error      -  Maximum number (384) of shader global keywords exceeded, keyword _FOG_EXP2 will be ignored.", "Unknown", "", ""},
		{"shader 384 valid timestamp", "2021.08.20 04:20:59 Error      -  Maximum number (384) of shader global keywords exceeded, keyword _FOG_EXP2 will be ignored.", "Event", "message", "Shader Keyword Limit has been reached"},
		{"join blocked", "2021.04.07 09:34:37 Error      -  [Behaviour] Master is not sending any events! Moving to a new instance.", "Event", "message", "Joining instance blocked by master"},
		{"avatar pedestal", "2021.05.07 10:48:19 Log        -  [Network Processing] RPC invoked SwitchAvatar on AvatarPedestal for User", "Event", "message", "User changed avatar pedestal"},
		{"video error", "2021.04.08 06:37:45 Error -  [Video Playback] ERROR: Video unavailable", "Event", "message", "VideoError: Video unavailable"},
		{"video private", "2021.04.08 06:40:07 Error -  [Video Playback] ERROR: Private video", "Event", "message", "VideoError: Private video"},
		{"avpro error", "2024.07.31 22:28:47 Error      -  [AVProVideo] Error: Loading failed.  File not found, codec not supported, video resolution too high or insufficient system resources.", "Event", "message", "VideoError: Loading failed.  File not found, codec not supported, video resolution too high or insufficient system resources."},
		{"untrusted url", "2025.05.04 22:38:12 Error      -  Attempted to play an untrusted URL (Domain: localhost) that is not allowlisted for public instances. If this URL is needed for the world to work, the domain needs to be added to the world's Video Player Allowed Domains list on the website.", "Event", "message", "VideoError: Attempted to play an untrusted URL (Domain: localhost) that is not allowlisted for public instances. If this URL is needed for the world to work, the domain needs to be added to the world's Video Player Allowed Domains list on the website."},
		{"vrcx world video", `2021.04.20 13:37:00 Log        -  [VRCX] VideoPlay(PyPyDance) "https://jd.pypy.moe/api/v1/videos/-Q3pdlsQxOk.mp4",0.5338666,260.6938,"1339 : Le Freak (Random)"`, "Event", "message", `VideoPlay(PyPyDance) "https://jd.pypy.moe/api/v1/videos/-Q3pdlsQxOk.mp4",0.5338666,260.6938,"1339 : Le Freak (Random)"`},
		{"vrcx world data ignored", "2021.04.20 13:37:01 Log        -  [VRCX-World] store:test:testvalue", "", "", ""},
		{"video attempting", "2021.04.20 13:37:59 Log        -  [Video Playback] Attempting to resolve URL 'https://www.youtube.com/watch?v=dQw4w9WgXcQ'", "VideoPlay", "videoUrl", "https://www.youtube.com/watch?v=dQw4w9WgXcQ"},
		{"video resolving", "2023.05.12 15:53:48 Log        -  [Video Playback] Resolving URL 'rtspt://topaz.chat/live/kiriri520'", "VideoPlay", "videoUrl", "rtspt://topaz.chat/live/kiriri520"},
		{"sdk2 video", "2021.04.23 13:12:25 Log        -  User Natsumi-sama added URL https://www.youtube.com/watch?v=dQw4w9WgXcQ", "VideoPlay", "displayName", "Natsumi-sama"},
		{"usharp video", "2021.12.12 05:51:58 Log        -  [USharpVideo] Started video load for URL: https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=1s, requested by ʜ ᴀ ᴘ ᴘ ʏ", "VideoPlay", "displayName", "ʜ ᴀ ᴘ ᴘ ʏ"},
		{"usharp sync", "2022.01.16 05:20:23 Log        -  [USharpVideo] Syncing video to 2.52", "Event", "message", "Video sync: 2.52"},
		{"notification", `2021.01.03 05:48:58 Log        -  [API] Received Notification: < Notification from username:pypy, sender user id:usr_4f76a584-9d4b-46f6-8209-8305eb683661 to of type: friendRequest, id: not_3a8f66eb-613c-4351-bee3-9980e6b5652c, created at: 01/14/2021 15:38:40 UTC, details: {{}}, type:friendRequest, m seen:False, message: ""> received at 01/02/2021 16:48:58 UTC`, "Event", "eventType", "Notification"},
		{"api request worlds", "2021.10.03 09:49:50 Log        -  [API] [110] Sending Get request to https://api.vrchat.cloud/api/1/worlds?apiKey=JlE5Jldo5Jibnk5O5hTx6XVqsJu4WJ26&organization=vrchat&userId=usr_032383a7-748c-4fb2-94e4-bcb928e5de6b&n=99&order=descending&offset=0&releaseStatus=public&maxUnityVersion=2019.4.31f1&minUnityVersion=5.5.0f1&maxAssetVersion=4&minAssetVersion=0&platform=standalonewindows", "Event", "eventType", "APIRequest"},
		{"api request user", "2021.10.03 09:48:43 Log        -  [API] [101] Sending Get request to https://api.vrchat.cloud/api/1/users/usr_032383a7-748c-4fb2-94e4-bcb928e5de6b?apiKey=JlE5Jldo5Jibnk5O5hTx6XVqsJu4WJ26&organization=vrchat", "Event", "eventType", "APIRequest"},
		{"avatar change", "2023.11.05 14:45:57 Log        -  [Behaviour] Switching K․MOG to avatar MoeSera", "Event", "eventType", "AvatarChange"},
		{"photon config commented out", "2021.11.02 02:21:41 Log        -  [Behaviour] Configuring remote player VRCPlayer[Remote] 22349737 1194", "Unknown", "", ""},
		{"photon initialized commented out", "2021.11.02 02:21:41 Log        -  [Behaviour] Initialized player Natsumi-sama", "Unknown", "", ""},
		{"photon limb remote commented out", "2021.11.10 08:10:28 Log        -  [Behaviour] Initialize Limb Avatar (UnityEngine.Animator) VRCPlayer[Remote] 78614426 59 (ǄǄǄǅǄǅǅǄǅǄǄǅǅǄǅǄǅǅǅǄǄǄǅǄǄǅǅǄǅǅǄǅǅǄǅǅǅǅǄǅǄǅǄǄǄǄǅ) False Loading", "Unknown", "", ""},
		{"photon limb local commented out", "2021.11.10 08:57:32 Log        -  [Behaviour] Initialize Limb Avatar (UnityEngine.Animator) VRCPlayer[Local] 59136629 1 (ǄǄǄǅǄǅǅǄǅǄǄǅǅǄǅǄǅǅǅǄǄǄǅǄǄǅǅǄǅǅǄǅǅǄǅǅǅǅǄǅǄǅǄǄǄǄǅ) True Loading", "Unknown", "", ""},
		{"photon three point commented out", "2022.03.05 11:29:16 Log        -  [Behaviour] Initialize ThreePoint Avatar (UnityEngine.Animator) VRCPlayer[Local] 50608765 1 (ǄǅǄǄǄǅǄǅǅǄǅǄǄǅǅǄǄǄǅǄǄǄǅǄǅǄǅǅǄǄǄǄǅǅǄǄǄǄǅǅǄǄǅǄǄǅǅ) True Custom", "Unknown", "", ""},
		{"audio initial device", "2022.03.15 03:40:34 Log        -  [Always] uSpeak: SetInputDevice 0 (3 total) 'Index HMD Mic (Valve VR Radio & HMD Mic)'", "", "", ""},
		{"audio changed", "2022.03.15 04:02:22 Log        -  [Always] uSpeak: OnAudioConfigurationChanged - devicesChanged = True, resetting mic..", "", "", ""},
		{"audio by name ignored", "2022.03.15 04:02:22 Log        -  [Always] uSpeak: SetInputDevice by name 'Index HMD Mic (Valve VR Radio & HMD Mic)' (3 total)", "Unknown", "", ""},
		{"audio new device", "2025.01.03 19:11:42 Log        -  [Always] uSpeak: SetInputDevice 0 (2 total) 'Microphone (NVIDIA Broadcast)'", "Event", "message", "Audio device changed, mic set to 'Microphone (NVIDIA Broadcast)'"},
		{"udon header", "2022.11.29 04:27:33 Error      -  [UdonBehaviour] An exception occurred during Udon execution, this UdonBehaviour will be halted.", "Unknown", "", ""},
		{"udon vm", "VRC.Udon.VM.UdonVMException: An exception occurred in an UdonVM, execution will be halted. ---> VRC.Udon.VM.UdonVMException: An exception occurred during EXTERN to 'VRCSDKBaseVRCPlayerApi.__get_displayName__SystemString'. ---> System.NullReferenceException: Object reference not set to an instance of an object.", "Event", "eventType", "UdonException"},
		{"application quit", "2022.06.12 01:51:46 Log        -  VRCApplication: OnApplicationQuit at 1603.499", "Event", "eventType", "VRCQuit"},
		{"application handle quit", "2024.10.23 21:18:34 Log        -  VRCApplication: HandleApplicationQuit at 936.5161", "Event", "eventType", "VRCQuit"},
		{"openvr initialized ignored", "2022.07.29 02:52:14 Log        -  OpenVR initialized!", "Unknown", "", ""},
		{"initializing vrsdk", "2023.04.22 16:52:28 Log        -  Initializing VRSDK.", "Event", "eventType", "OpenVRInit"},
		{"start vrsdk ignored", "2023.04.22 16:52:29 Log        -  StartVRSDK: Open VR Loader", "Unknown", "", ""},
		{"steamvr model", "2024.07.26 01:48:56 Log        -  STEAMVR HMD Model: Index", "Event", "eventType", "OpenVRInit"},
		{"desktop mode", "2023.04.22 16:54:18 Log        -  VR Disabled", "Event", "eventType", "DesktopMode"},
		{"string load", "2023.03.23 11:37:21 Log        -  [String Download] Attempting to load String from URL 'https://pastebin.com/raw/BaW6NL2L'", "ResourceLoad", "resourceType", "StringLoad"},
		{"image load", "2023.03.23 11:32:25 Log        -  [Image Download] Attempting to load image from URL 'https://i.imgur.com/lCfUMX0.jpeg'", "ResourceLoad", "resourceType", "ImageLoad"},
		{"vote kicked", "2023.06.02 01:08:04 Log        -  [Behaviour] Received executive message: You have been kicked from the instance by majority vote", "Event", "message", "You have been kicked from the instance by majority vote"},
		{"kicked hour ignored", "2023.06.02 01:11:58 Log        -  [Behaviour] You have been kicked from this world for an hour.", "Unknown", "", ""},
		{"failed join", "2023.09.01 10:42:19 Warning    -  [Behaviour] Failed to join instance 'wrld_78eb6b52-fd5a-4954-ba28-972c92c8cc77:82384~hidden(usr_a9bf892d-b447-47ce-a572-20c83dbfffd8)~region(eu)' due to 'That instance is using an outdated version of VRChat. You won't be able to join them until they update!'", "Event", "message", "'wrld_78eb6b52-fd5a-4954-ba28-972c92c8cc77:82384~hidden(usr_a9bf892d-b447-47ce-a572-20c83dbfffd8)~region(eu)' due to 'That instance is using an outdated version of VRChat. You won't be able to join them until they update!'"},
		{"osc failed", "2023.09.26 04:12:57 Warning    -  Could not Start OSC: Address already in use", "Event", "message", `VRChat couldn't start OSC server, "Could not Start OSC: Address already in use"`},
		{"instance reset", "2024.08.30 01:43:40 Log        -  [ModerationManager] This instance will be reset in 60 minutes due to its age.", "Event", "message", "This instance will be reset in 60 minutes due to its age."},
		{"vote kick initiation", "2024.08.29 02:04:47 Log        -  [ModerationManager] A vote kick has been initiated against בורקס במאווררים 849d, do you agree?", "Event", "message", "A vote kick has been initiated against בורקס במאווררים 849d, do you agree?"},
		{"vote kick success", "2024.08.29 02:05:21 Log        -  [ModerationManager] Vote to kick בורקס במאווררים 849d succeeded", "Event", "message", "Vote to kick בורקס במאווררים 849d succeeded"},
		{"sticker spawn", "2024.08.29 02:05:22 Log        -  [StickersManager] User usr_032383a7-748c-4fb2-94e4-bcb928e5de6b (Natsumi-sama) spawned sticker inv_8b380ee4-9a8a-484e-a0c3-b01290b92c6a", "Event", "eventType", "StickerSpawn"},
	}

	for i, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := ParseLine("output_log.txt", int64(i), tt.line, ctx)
			if tt.expected == "" {
				if ok || e != nil {
					t.Fatalf("expected consumed/ignored line, got ok=%v entry=%+v", ok, e)
				}
				return
			}
			if !ok || e == nil {
				t.Fatalf("expected %s entry", tt.expected)
			}
			if e.Type != tt.expected {
				t.Fatalf("type=%q want %q", e.Type, tt.expected)
			}
			if tt.key != "" {
				if got := e.Payload[tt.key]; got != tt.want {
					t.Fatalf("payload[%s]=%v want %q", tt.key, got, tt.want)
				}
			}
			if strings.TrimSpace(e.RawLine) == "" {
				t.Fatalf("raw line was not preserved")
			}
		})
	}
}

func TestParseLineDeduplicatesVideoErrorsAndIgnoresOwnResourceLoads(t *testing.T) {
	ctx := NewFileContext()
	line := "2024.07.31 22:28:47 Error      -  [AVProVideo] Error: Loading failed."
	first, ok := ParseLine("output_log.txt", 1, line, ctx)
	if !ok || first == nil {
		t.Fatalf("expected first video error")
	}
	second, ok := ParseLine("output_log.txt", 2, line, ctx)
	if ok || second != nil {
		t.Fatalf("expected duplicate video error to be suppressed")
	}

	for _, line := range []string{
		"2026.06.18 12:00:00 Log        -  [String Download] Attempting to load String from URL 'http://127.0.0.1:22500/foo'",
		"2026.06.18 12:00:01 Log        -  [Image Download] Attempting to load image from URL 'http://localhost:22500/foo.png'",
	} {
		e, ok := ParseLine("output_log.txt", 3, line, ctx)
		if ok || e != nil {
			t.Fatalf("expected own resource load to be ignored, got ok=%v entry=%+v", ok, e)
		}
	}
}
