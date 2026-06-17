package parser

import "testing"

func TestParseLineKnownEvents(t *testing.T) {
	ctx := NewFileContext()
	cases := []struct {
		name string
		line string
		typ  string
		key  string
		want string
	}{
		{
			name: "location",
			line: "2026.06.18 12:00:00 Log        -  [Behaviour] Joining wrld_abc:12345~public~region(us)",
			typ:  "Location",
			key:  "location",
			want: "wrld_abc:12345~public~region(us)",
		},
		{
			name: "join",
			line: "2026.06.18 12:00:01 Log        -  [Behaviour] OnPlayerJoined Alice (usr_123)",
			typ:  "OnPlayerJoined",
			key:  "userId",
			want: "usr_123",
		},
		{
			name: "video",
			line: "2026.06.18 12:00:02 Log        -  [Video Playback] Resolving URL 'https://example.test/video.mp4'",
			typ:  "VideoPlay",
			key:  "videoUrl",
			want: "https://example.test/video.mp4",
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			e, ok := ParseLine("output_log.txt", 10, tt.line, ctx)
			if !ok || e == nil {
				t.Fatalf("expected entry")
			}
			if e.Type != tt.typ {
				t.Fatalf("type=%q want %q", e.Type, tt.typ)
			}
			if got := e.Payload[tt.key]; got != tt.want {
				t.Fatalf("payload[%s]=%v want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestParseLineUnknownAndIgnore(t *testing.T) {
	ctx := NewFileContext()
	ignored, ok := ParseLine("output_log.txt", 10,
		"2026.06.18 12:00:00 Log        -  [String Download] Attempting to load String from URL 'http://127.0.0.1:22500/foo'",
		ctx,
	)
	if ok || ignored != nil {
		t.Fatalf("expected localhost resource load to be ignored")
	}

	e, ok := ParseLine("output_log.txt", 20,
		"2026.06.18 12:00:01 Log        -  [NewVRChatThing] Changed format",
		ctx,
	)
	if !ok || e == nil {
		t.Fatalf("expected unknown entry")
	}
	if e.Type != "Unknown" {
		t.Fatalf("type=%q want Unknown", e.Type)
	}
}
