package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/owainlewis/passage-cli/internal/config"
)

const appName = "passage"

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type Runtime struct {
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	ConfigDir string
	Env       map[string]string
	HTTP      *http.Client
	Build     BuildInfo
}

func Run(args []string, stdout io.Writer, stderr io.Writer, build BuildInfo) int {
	dir, err := config.DefaultDir()
	if err != nil {
		fmt.Fprintf(stderr, "%s: %v\n", appName, err)
		return 1
	}
	return RunWithRuntime(args, Runtime{
		Stdin:     os.Stdin,
		Stdout:    stdout,
		Stderr:    stderr,
		ConfigDir: dir,
		Env:       config.EnvMap(),
		HTTP:      http.DefaultClient,
		Build:     build,
	})
}

func RunWithRuntime(args []string, rt Runtime) int {
	if rt.Stdin == nil {
		rt.Stdin = strings.NewReader("")
	}
	if rt.Stdout == nil {
		rt.Stdout = io.Discard
	}
	if rt.Stderr == nil {
		rt.Stderr = io.Discard
	}
	if rt.Env == nil {
		rt.Env = map[string]string{}
	}
	if rt.HTTP == nil {
		rt.HTTP = http.DefaultClient
	}
	if len(args) == 0 {
		printHelp(rt.Stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(rt.Stdout)
		return 0
	case "login":
		return runLogin(rt)
	case "auth":
		return runAuth(args[1:], rt)
	case "version", "-v", "--version":
		printVersion(rt.Stdout, rt.Build)
		return 0
	default:
		fmt.Fprintf(rt.Stderr, "%s: unknown command %q\n", appName, args[0])
		fmt.Fprintf(rt.Stderr, "Run `%s help` for usage.\n", appName)
		return 1
	}
}

func runLogin(rt Runtime) int {
	reader := bufio.NewReader(rt.Stdin)
	apiURL, ok := prompt(reader, rt.Stdout, "API URL", config.DefaultAPIURL)
	if !ok {
		fmt.Fprintln(rt.Stderr, "login canceled")
		return 1
	}
	token, ok := prompt(reader, rt.Stdout, "API token", "")
	if !ok {
		fmt.Fprintln(rt.Stderr, "login canceled")
		return 1
	}
	cfg := config.Config{APIURL: apiURL, Token: token}
	if err := config.Save(rt.ConfigDir, cfg); err != nil {
		fmt.Fprintf(rt.Stderr, "%s: login failed: %v\n", appName, err)
		return 1
	}
	fmt.Fprintf(rt.Stdout, "Saved credentials for %s\n", strings.TrimRight(apiURL, "/"))
	fmt.Fprintf(rt.Stdout, "Token %s\n", config.RedactToken(token))
	return 0
}

func runAuth(args []string, rt Runtime) int {
	if len(args) == 0 {
		return runAuthStatus(rt, false)
	}
	if args[0] == "status" {
		check := len(args) > 1 && args[1] == "--check"
		if len(args) > 2 || (len(args) == 2 && !check) {
			fmt.Fprintf(rt.Stderr, "%s: usage: passage auth status [--check]\n", appName)
			return 1
		}
		return runAuthStatus(rt, check)
	}
	fmt.Fprintf(rt.Stderr, "%s: unknown auth command %q\n", appName, args[0])
	fmt.Fprintf(rt.Stderr, "Run `%s help` for usage.\n", appName)
	return 1
}

func runAuthStatus(rt Runtime, check bool) int {
	loaded, err := config.Load(rt.ConfigDir, rt.Env)
	if err != nil {
		fmt.Fprintf(rt.Stderr, "%s: %v\n", appName, err)
		return 1
	}
	if loaded.Config.Token == "" {
		fmt.Fprintln(rt.Stderr, "Not authenticated. Run `passage login` or set PASSAGE_TOKEN.")
		return 1
	}

	fmt.Fprintln(rt.Stdout, "Authenticated")
	fmt.Fprintf(rt.Stdout, "API URL: %s (%s)\n", loaded.Config.APIURL, loaded.Source.APIURL)
	fmt.Fprintf(rt.Stdout, "Token: %s (%s)\n", config.RedactToken(loaded.Config.Token), loaded.Source.Token)
	if check {
		user, err := checkAuth(rt.HTTP, loaded.Config)
		if err != nil {
			fmt.Fprintf(rt.Stderr, "%s: auth check failed: %v\n", appName, err)
			return 1
		}
		fmt.Fprintf(rt.Stdout, "Server: authenticated as %s\n", user.Email)
	}
	return 0
}

func checkAuth(client *http.Client, cfg config.Config) (meUser, error) {
	req, err := http.NewRequest(http.MethodGet, cfg.APIURL+"/api/v1/me", nil)
	if err != nil {
		return meUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	res, err := client.Do(req)
	if err != nil {
		return meUser{}, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return meUser{}, err
	}
	if res.StatusCode == http.StatusUnauthorized {
		return meUser{}, errors.New("authentication required")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return meUser{}, fmt.Errorf("server returned %d", res.StatusCode)
	}
	var me meResponse
	if err := json.Unmarshal(body, &me); err != nil {
		return meUser{}, err
	}
	if !me.Authenticated || me.User == nil {
		return meUser{}, errors.New("not authenticated")
	}
	return *me.User, nil
}

func prompt(reader *bufio.Reader, stdout io.Writer, label string, fallback string) (string, bool) {
	if fallback == "" {
		fmt.Fprintf(stdout, "%s: ", label)
	} else {
		fmt.Fprintf(stdout, "%s [%s]: ", label, fallback)
	}
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return value, true
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
passage is the command line client for passage.md.

Usage:
  passage <command>

Commands:
  login     Save API URL and token.
  auth      Show auth status.
  help      Show this help.
  version   Show build version.

More commands are coming in the Phase 2 agent access work.
`)+"\n")
}

func printVersion(w io.Writer, build BuildInfo) {
	fmt.Fprintf(w, "passage %s\n", build.Version)
	if build.Commit != "" && build.Commit != "unknown" {
		fmt.Fprintf(w, "commit %s\n", build.Commit)
	}
	if build.Date != "" && build.Date != "unknown" {
		fmt.Fprintf(w, "built %s\n", build.Date)
	}
}

type meResponse struct {
	Authenticated bool    `json:"authenticated"`
	User          *meUser `json:"user"`
}

type meUser struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
