// Command log-agent tails VRChat logs on Windows and uploads GameLog entries.
package main

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/gamelog"
	"github.com/lemolatoon/vrcx-mobile/vrcx-server/internal/gamelog/parser"
)

type Config struct {
	ServerURL string `json:"server_url"`
	Token     string `json:"token"`
	LogDir    string `json:"log_dir"`
	SourceID  string `json:"source_id"`
}

type State struct {
	Files    map[string]int64 `json:"files"`
	LastSent string           `json:"last_sent,omitempty"`
}

type uploadBody struct {
	SourceID string          `json:"source_id"`
	Entries  []gamelog.Entry `json:"entries"`
}

var fileContexts = map[string]*parser.FileContext{}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "setup":
		err = setup(os.Args[2:])
	case "run":
		err = run()
	case "status":
		err = status()
	case "install-startup":
		err = installStartup()
	case "uninstall-startup":
		err = uninstallStartup()
	case "tail-once":
		err = tailOnce(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func setup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ExitOnError)
	server := fs.String("server", "", "VRCX Mobile server URL")
	token := fs.String("token", "", "agent token")
	logDir := fs.String("log-dir", defaultVRChatLogDir(), "VRChat log directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*server) == "" || strings.TrimSpace(*token) == "" {
		return errors.New("setup requires --server and --token")
	}
	sourceID, err := randomHex(16)
	if err != nil {
		return err
	}
	cfg := Config{
		ServerURL: strings.TrimRight(strings.TrimSpace(*server), "/"),
		Token:     strings.TrimSpace(*token),
		LogDir:    *logDir,
		SourceID:  sourceID,
	}
	if err := os.MkdirAll(filepath.Dir(configPath()), 0o700); err != nil {
		return err
	}
	if err := writeJSON(configPath(), cfg); err != nil {
		return err
	}
	if err := initStateAtEnd(cfg); err != nil {
		return err
	}
	if err := upload(cfg, nil); err != nil {
		return fmt.Errorf("config saved, but server validation failed: %w", err)
	}
	fmt.Println("Configured VRCX Mobile Log Agent")
	fmt.Println("Config:", configPath())
	fmt.Println("State:", statePath())
	return nil
}

func run() error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(logPath()), 0o700); err == nil {
		f, ferr := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
		if ferr == nil {
			defer f.Close()
			log.SetOutput(io.MultiWriter(os.Stderr, f))
		}
	}
	log.Printf("starting source=%s log_dir=%s server=%s", cfg.SourceID, cfg.LogDir, cfg.ServerURL)
	backoff := 2 * time.Second
	for {
		if err := drainQueue(cfg); err != nil {
			log.Printf("queue drain failed: %v", err)
			time.Sleep(backoff)
			backoff = nextBackoff(backoff)
			continue
		}
		entries, err := collect(cfg, false)
		if err != nil {
			log.Printf("collect failed: %v", err)
			time.Sleep(2 * time.Second)
			continue
		}
		if len(entries) > 0 {
			if err := upload(cfg, entries); err != nil {
				log.Printf("upload failed: %v", err)
				if qerr := appendQueue(entries); qerr != nil {
					log.Printf("queue append failed: %v", qerr)
				}
				time.Sleep(backoff)
				backoff = nextBackoff(backoff)
				continue
			}
			backoff = 2 * time.Second
			st, _ := readState()
			st.LastSent = time.Now().UTC().Format(time.RFC3339)
			_ = writeJSON(statePath(), st)
		}
		time.Sleep(time.Second)
	}
}

func status() error {
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	st, _ := readState()
	queueBytes := int64(0)
	if info, err := os.Stat(queuePath()); err == nil {
		queueBytes = info.Size()
	}
	fmt.Println("Config:", configPath())
	fmt.Println("State:", statePath())
	fmt.Println("Queue:", queuePath())
	fmt.Println("Agent log:", logPath())
	fmt.Println("Server:", cfg.ServerURL)
	fmt.Println("Log dir:", cfg.LogDir)
	fmt.Println("Source ID:", cfg.SourceID)
	fmt.Println("Tracked files:", len(st.Files))
	fmt.Println("Queue bytes:", queueBytes)
	fmt.Println("Last sent:", st.LastSent)
	return nil
}

func installStartup() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tr := fmt.Sprintf(`"%s" run`, exe)
	cmd := exec.Command("schtasks", "/Create", "/F", "/TN", "VRCX Mobile Log Agent", "/SC", "ONLOGON", "/TR", tr)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks create: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fmt.Print(string(out))
	return nil
}

func uninstallStartup() error {
	cmd := exec.Command("schtasks", "/Delete", "/F", "/TN", "VRCX Mobile Log Agent")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("schtasks delete: %w: %s", err, strings.TrimSpace(string(out)))
	}
	fmt.Print(string(out))
	return nil
}

func tailOnce(args []string) error {
	fs := flag.NewFlagSet("tail-once", flag.ExitOnError)
	sinceStart := fs.Bool("since-start", false, "read from start instead of saved offsets")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := readConfig()
	if err != nil {
		return err
	}
	entries, err := collect(cfg, *sinceStart)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(entries)
}

func collect(cfg Config, sinceStart bool) ([]gamelog.Entry, error) {
	st, err := readState()
	if err != nil {
		return nil, err
	}
	if st.Files == nil {
		st.Files = map[string]int64{}
	}
	files, err := filepath.Glob(filepath.Join(cfg.LogDir, "output_log_*.txt"))
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	var entries []gamelog.Entry
	for _, path := range files {
		name := filepath.Base(path)
		offset := st.Files[name]
		if sinceStart {
			offset = 0
		}
		ctx := fileContexts[name]
		if ctx == nil || sinceStart {
			ctx = parser.NewFileContext()
			fileContexts[name] = ctx
		}
		fileEntries, newOffset, err := parseFile(path, name, offset, ctx)
		if err != nil {
			log.Printf("parse %s failed: %v", name, err)
			continue
		}
		entries = append(entries, fileEntries...)
		if !sinceStart {
			st.Files[name] = newOffset
		}
		if len(entries) >= 100 {
			entries = entries[:100]
			break
		}
	}
	if !sinceStart {
		if err := writeJSON(statePath(), st); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

func parseFile(path, name string, offset int64, ctx *parser.FileContext) ([]gamelog.Entry, int64, error) {
	if ctx == nil {
		ctx = parser.NewFileContext()
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, offset, err
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, offset, err
	}
	reader := bufio.NewReader(f)
	var out []gamelog.Entry
	pos := offset
	for {
		lineOffset := pos
		line, err := reader.ReadString('\n')
		pos += int64(len(line))
		if len(line) > 0 {
			if e, ok := parser.ParseLine(name, lineOffset, line, ctx); ok && e != nil {
				out = append(out, *e)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return out, pos, err
		}
	}
	return out, pos, nil
}

func initStateAtEnd(cfg Config) error {
	files, err := filepath.Glob(filepath.Join(cfg.LogDir, "output_log_*.txt"))
	if err != nil {
		return err
	}
	st := State{Files: map[string]int64{}}
	for _, path := range files {
		info, err := os.Stat(path)
		if err == nil {
			st.Files[filepath.Base(path)] = info.Size()
		}
	}
	if err := os.MkdirAll(filepath.Dir(statePath()), 0o700); err != nil {
		return err
	}
	return writeJSON(statePath(), st)
}

func upload(cfg Config, entries []gamelog.Entry) error {
	body, _ := json.Marshal(uploadBody{SourceID: cfg.SourceID, Entries: entries})
	req, err := http.NewRequest(http.MethodPost, cfg.ServerURL+"/api/v1/gamelog/ingest", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("server status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

func appendQueue(entries []gamelog.Entry) error {
	if len(entries) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(queuePath()), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(queuePath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

func drainQueue(cfg Config) error {
	path := queuePath()
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var entries []gamelog.Entry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e gamelog.Entry
		if err := json.Unmarshal(scanner.Bytes(), &e); err == nil {
			entries = append(entries, e)
		}
		if len(entries) >= 100 {
			break
		}
	}
	f.Close()
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.Remove(path)
	}
	if err := upload(cfg, entries); err != nil {
		return err
	}
	return rewriteQueueWithout(entries)
}

func rewriteQueueWithout(sent []gamelog.Entry) error {
	path := queuePath()
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	tmp := path + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	skip := len(sent)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if skip > 0 {
			skip--
			continue
		}
		if _, err := out.Write(append(scanner.Bytes(), '\n')); err != nil {
			return err
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	out.Close()
	return os.Rename(tmp, path)
}

func readConfig() (Config, error) {
	var cfg Config
	if err := readJSON(configPath(), &cfg); err != nil {
		return cfg, fmt.Errorf("read config: %w; run setup first", err)
	}
	return cfg, nil
}

func readState() (State, error) {
	var st State
	if err := readJSON(statePath(), &st); err != nil {
		if os.IsNotExist(err) {
			return State{Files: map[string]int64{}}, nil
		}
		return st, err
	}
	if st.Files == nil {
		st.Files = map[string]int64{}
	}
	return st, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

func defaultVRChatLogDir() string {
	if runtime.GOOS == "windows" {
		if localLow := os.Getenv("USERPROFILE"); localLow != "" {
			return filepath.Join(localLow, "AppData", "LocalLow", "VRChat", "VRChat")
		}
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "AppData", "LocalLow", "VRChat", "VRChat")
}

func configPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		base, _ = os.UserConfigDir()
	}
	return filepath.Join(base, "VRCX-Mobile", "log-agent", "config.json")
}

func dataDir() string {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base, _ = os.UserCacheDir()
	}
	return filepath.Join(base, "VRCX-Mobile", "log-agent")
}

func statePath() string { return filepath.Join(dataDir(), "state.json") }
func queuePath() string { return filepath.Join(dataDir(), "queue.jsonl") }
func logPath() string   { return filepath.Join(dataDir(), "agent.log") }

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func nextBackoff(d time.Duration) time.Duration {
	d *= 2
	if d > 5*time.Minute {
		return 5 * time.Minute
	}
	return d
}

func usage() {
	fmt.Println(`Usage:
  vrcx-log-agent setup --server <url> --token <token> [--log-dir <path>]
  vrcx-log-agent run
  vrcx-log-agent status
  vrcx-log-agent install-startup
  vrcx-log-agent uninstall-startup
  vrcx-log-agent tail-once [--since-start]`)
}
