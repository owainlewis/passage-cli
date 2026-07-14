package cli

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/owainlewis/passage-cli/internal/api"
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
	case "new":
		return runNew(args[1:], rt)
	case "list":
		return runList(args[1:], rt)
	case "cat":
		return runCat("cat", args[1:], rt)
	case "pull":
		return runCat("pull", args[1:], rt)
	case "push", "replace":
		return runReplace(args[0], args[1:], rt)
	case "append":
		return runAppend(args[1:], rt)
	case "delete":
		return runDelete(args[1:], rt)
	case "share":
		return runShare(args[1:], rt)
	case "raw":
		return runRaw(args[1:], rt)
	case "unshare":
		return runUnshare(args[1:], rt)
	case "version", "-v", "--version":
		printVersion(rt.Stdout, rt.Build)
		return 0
	default:
		fmt.Fprintf(rt.Stderr, "%s: unknown command %q\n", appName, args[0])
		fmt.Fprintf(rt.Stderr, "Run `%s help` for usage.\n", appName)
		return 1
	}
}

func runNew(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage new [--json] \"Title\"\n", appName)
		return 1
	}
	title := strings.TrimSpace(args[0])
	if title == "" {
		fmt.Fprintf(rt.Stderr, "%s: title is required\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	doc, err := client.Create("# " + title + "\n")
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	return printDocumentResult(rt.Stdout, doc, jsonOut, "Created")
}

func runList(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 0 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage list [--json]\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	docs, err := client.List()
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, map[string][]api.Document{"documents": docs})
	}
	for _, doc := range docs {
		fmt.Fprintf(rt.Stdout, "%s\t%s\t%s\n", doc.ID, doc.UpdatedAt.Format("2006-01-02 15:04"), doc.Title)
	}
	return 0
}

func runCat(command string, args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage %s [--json] <doc>\n", appName, command)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	doc, err := client.Get(args[0])
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, doc)
	}
	fmt.Fprint(rt.Stdout, doc.Body)
	return 0
}

func runReplace(command string, args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 2 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage %s [--json] <doc> <file>\n", appName, command)
		return 1
	}
	body, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(rt.Stderr, "%s: %v\n", appName, err)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	doc, err := client.Update(args[0], string(body))
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	return printDocumentResult(rt.Stdout, doc, jsonOut, "Updated")
}

func runAppend(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 2 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage append [--json] <doc> <file>\n", appName)
		return 1
	}
	addition, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Fprintf(rt.Stderr, "%s: %v\n", appName, err)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	existing, err := client.Get(args[0])
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	body := existing.Body
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += string(addition)
	doc, err := client.Update(args[0], body)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	return printDocumentResult(rt.Stdout, doc, jsonOut, "Updated")
}

func runDelete(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage delete [--json] <doc>\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if err := client.Delete(args[0]); err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, map[string]any{"deleted": true, "doc_id": args[0]})
	}
	fmt.Fprintf(rt.Stdout, "Deleted %s\n", args[0])
	return 0
}

func runShare(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage share [--json] <doc>\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	out, err := shareDocument(client, args[0])
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, out)
	}
	fmt.Fprintf(rt.Stdout, "Shared %s\n", out.DocID)
	fmt.Fprintf(rt.Stdout, "HTML: %s\n", out.HTMLURL)
	fmt.Fprintf(rt.Stdout, "Raw: %s\n", out.RawURL)
	return 0
}

func runRaw(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage raw [--json] <doc>\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	out, err := shareDocument(client, args[0])
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, out)
	}
	fmt.Fprintln(rt.Stdout, out.RawURL)
	return 0
}

func runUnshare(args []string, rt Runtime) int {
	jsonOut, args := parseJSONFlag(args)
	if len(args) != 1 {
		fmt.Fprintf(rt.Stderr, "%s: usage: passage unshare [--json] <doc>\n", appName)
		return 1
	}
	client, err := documentClient(rt)
	if err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if err := client.Unshare(args[0]); err != nil {
		printCommandError(rt.Stderr, err)
		return 1
	}
	if jsonOut {
		return printJSON(rt.Stdout, map[string]any{"doc_id": args[0], "unshared": true})
	}
	fmt.Fprintf(rt.Stdout, "Unshared %s\n", args[0])
	return 0
}

func documentClient(rt Runtime) (api.Client, error) {
	loaded, err := config.Load(rt.ConfigDir, rt.Env)
	if err != nil {
		return api.Client{}, err
	}
	return api.Client{BaseURL: loaded.Config.APIURL, Token: loaded.Config.Token, HTTP: rt.HTTP}, nil
}

func shareDocument(client api.Client, docID string) (shareOutput, error) {
	share, err := client.Share(docID)
	if err != nil {
		return shareOutput{}, err
	}
	htmlURL, err := absoluteURL(client.BaseURL, share.HTMLPath)
	if err != nil {
		return shareOutput{}, err
	}
	rawURL, err := absoluteURL(client.BaseURL, share.MarkdownPath)
	if err != nil {
		return shareOutput{}, err
	}
	return shareOutput{
		DocID:   docID,
		Token:   share.Token,
		HTMLURL: htmlURL,
		RawURL:  rawURL,
	}, nil
}

func absoluteURL(baseURL string, path string) (string, error) {
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil {
		return "", err
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(rel).String(), nil
}

func parseJSONFlag(args []string) (bool, []string) {
	var out []string
	jsonOut := false
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		out = append(out, arg)
	}
	return jsonOut, out
}

func printDocumentResult(w io.Writer, doc api.Document, jsonOut bool, verb string) int {
	if jsonOut {
		return printJSON(w, doc)
	}
	fmt.Fprintf(w, "%s %s\t%s\n", verb, doc.ID, doc.Title)
	return 0
}

func printJSON(w io.Writer, value any) int {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return 1
	}
	return 0
}

func printCommandError(w io.Writer, err error) {
	if err.Error() == "not authenticated" {
		fmt.Fprintln(w, "Not authenticated. Run `passage login` or set PASSAGE_TOKEN.")
		return
	}
	fmt.Fprintf(w, "%s: %v\n", appName, err)
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
  new       Create a document.
  list      List documents.
  cat       Print a document body.
  pull      Print a document body.
  push      Replace a document body from a file.
  append    Append file content to a document.
  replace   Replace a document body from a file.
  delete    Delete a document.
  share     Share a document and print public URLs.
  raw       Share a document and print the raw Markdown URL.
  unshare   Revoke public access for a document.
  help      Show this help.
  version   Show build version.
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

type shareOutput struct {
	DocID   string `json:"doc_id"`
	Token   string `json:"token"`
	HTMLURL string `json:"html_url"`
	RawURL  string `json:"raw_url"`
}
