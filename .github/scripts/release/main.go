// release builds and deploys SAM without a registry or a toolchain on the host.
package main

import (
	"archive/tar"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const reserve = uint64(1 << 30)

type object = map[string]any

type buildConfig struct {
	Common         map[string]string            `json:"common"`
	Services       map[string]map[string]string `json:"services"`
	Infrastructure map[string]string            `json:"infrastructure"`
}

type manifest struct {
	Release    string            `json:"release"`
	Root       string            `json:"root"`
	Images     []string          `json:"images"`
	ImageIDs   map[string]string `json:"image_ids"`
	ImageBytes uint64            `json:"image_bytes"`
	Bootstrap  bool              `json:"bootstrap,omitempty"`
}

type state struct {
	Current  string `json:"current"`
	Previous string `json:"previous,omitempty"`
}

type options struct {
	root, release, output, repo, archive, checksum string
	archiveBytes                                   uint64
	production                                     bool
}

var releasePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,100}$`)
var rootPattern = regexp.MustCompile(`^/[A-Za-z0-9_/-]+$`)

func main() {
	if err := mainError(); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}

func mainError() error {
	if len(os.Args) < 2 {
		return errors.New("expected prepare, pack, preflight, deploy or deliver")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	var o options
	flags.StringVar(&o.root, "root", "/opt/sam-backend", "existing backend root")
	flags.StringVar(&o.release, "release", "", "unique release ID")
	flags.StringVar(&o.output, "output", "", "release build directory")
	flags.StringVar(&o.repo, "repo", ".", "repository root")
	flags.StringVar(&o.archive, "archive", "", "release archive")
	flags.StringVar(&o.checksum, "sha256", "", "archive SHA256")
	flags.Uint64Var(&o.archiveBytes, "archive-bytes", 0, "incoming archive size")
	flags.BoolVar(&o.production, "production", false, "require BACKEND_CONFIG secret")
	if err := flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	if !rootPattern.MatchString(o.root) || o.root == "/" || filepath.Clean(o.root) != o.root {
		return errors.New("root must be an absolute clean path without spaces")
	}
	if (command == "prepare" || command == "deploy") && !releasePattern.MatchString(o.release) {
		return errors.New("invalid release ID")
	}
	switch command {
	case "prepare":
		return prepare(o)
	case "pack":
		return pack(o)
	case "preflight":
		return preflight(o)
	case "deploy":
		return deploy(o)
	case "deliver":
		return deliver()
	default:
		return errors.New("unknown release command")
	}
}

// Captured command output may contain configuration secrets. Never put it in errors.
func command(capture bool, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	if !capture {
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		return "", cmd.Run()
	}
	data, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s command failed (output withheld)", name)
	}
	return string(data), nil
}

func readJSON(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if json.Unmarshal(data, value) != nil {
		return fmt.Errorf("invalid JSON in %s", filepath.Base(path))
	}
	return nil
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return errors.New("cannot encode JSON")
	}
	if err := os.WriteFile(path+".tmp", append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func mapAt(m object, key string) object {
	value, _ := m[key].(map[string]any)
	if value == nil {
		return object{}
	}
	return value
}

func textAt(m object, key string) string {
	value, _ := m[key].(string)
	return value
}

func listAt(m object, key string) []any {
	value, _ := m[key].([]any)
	return value
}

func within(root, path string) (string, bool) {
	relative, err := filepath.Rel(root, path)
	return relative, err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))
}

func copyPath(source, target string) error {
	info, err := os.Lstat(source)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || (!info.IsDir() && !info.Mode().IsRegular()) {
		return errors.New("runtime assets must be regular files/directories")
	}
	if info.IsDir() {
		if err := os.MkdirAll(target, info.Mode().Perm()); err != nil {
			return err
		}
		entries, err := os.ReadDir(source)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if err := copyPath(filepath.Join(source, entry.Name()), filepath.Join(target, entry.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	in, err := os.Open(source)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, info.Mode().Perm())
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	return errors.Join(copyErr, out.Close())
}

func composeConfig(files ...string) (object, error) {
	args := []string{"compose", "-p", "sam"}
	for _, file := range files {
		args = append(args, "-f", file)
	}
	args = append(args, "config", "--no-interpolate", "--no-env-resolution", "--format", "json")
	data, err := command(true, "docker", args...)
	if err != nil {
		return nil, err
	}
	var config object
	if json.Unmarshal([]byte(data), &config) != nil {
		return nil, errors.New("cannot decode Compose configuration")
	}
	for key := range config {
		if strings.HasPrefix(key, "x-") {
			delete(config, key)
		}
	}
	return config, nil
}

func loadBuildConfig(required bool) (buildConfig, string, error) {
	var config buildConfig
	data := os.Getenv("BACKEND_CONFIG")
	if data == "" {
		if required {
			return config, "", errors.New("GitHub secret BACKEND_CONFIG is required")
		}
		return config, "", nil
	}
	decoder := json.NewDecoder(strings.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&config); err != nil {
		return config, "", errors.New("BACKEND_CONFIG must contain common, services and infrastructure JSON objects")
	}
	if config.Common["JWT_PRIVATE_PEM"] != "" {
		return config, "", errors.New("JWT_PRIVATE_PEM must not be distributed to backend services")
	}
	for _, key := range []string{"JWT_SERVICE_SECRET", "JWT_PUBLIC_KEY_FILE", "JWT_PUBLIC_PEM"} {
		if config.Common[key] == "" {
			return config, "", fmt.Errorf("BACKEND_CONFIG.common.%s is required", key)
		}
	}
	for _, key := range []string{"DOMAIN", "POSTGRES_PASSWORD", "INVENTORY_DB_PASSWORD", "LICENSE_DB_PASSWORD", "CHAT_DB_PASSWORD"} {
		if config.Infrastructure[key] == "" {
			return config, "", fmt.Errorf("BACKEND_CONFIG.infrastructure.%s is required", key)
		}
	}
	digest := sha256.Sum256([]byte(data))
	return config, hex.EncodeToString(digest[:]), nil
}

func prepare(o options) error {
	if o.output == "" || exists(o.output) {
		return errors.New("output must specify a new directory")
	}
	repo, err := filepath.Abs(o.repo)
	if err != nil {
		return err
	}
	config, err := composeConfig(filepath.Join(repo, "back/compose.yaml"),
		filepath.Join(repo, "back/compose.server.yaml"), filepath.Join(repo, ".github/compose.frontend.yaml"))
	if err != nil {
		return err
	}
	secrets, revision, err := loadBuildConfig(o.production)
	if err != nil {
		return err
	}
	payload := filepath.Join(o.output, "payload")
	if err := os.MkdirAll(payload, 0o700); err != nil {
		return err
	}
	serverRelease := filepath.Join(o.root, ".cd/releases", o.release)
	targets, assets := object{}, map[string]bool{}
	var names, images []string
	for name, value := range mapAt(config, "services") {
		service := value.(map[string]any)
		if len(listAt(service, "profiles")) != 0 || len(listAt(service, "env_file")) != 0 {
			return errors.New("production profiles/env_file are unsupported; use BACKEND_CONFIG")
		}
		if rawBuild, ok := service["build"]; ok {
			build := rawBuild.(map[string]any)
			tag := "sam/" + name + ":" + o.release
			target := object{"context": build["context"], "dockerfile": build["dockerfile"],
				"tags": []string{tag}, "platforms": []string{"linux/amd64"},
				"labels": object{"io.sam.cd": "true", "io.sam.release": o.release}}
			if textAt(target, "dockerfile") == "" {
				target["dockerfile"] = "Dockerfile"
			}
			for key, value := range build {
				switch key {
				case "context", "dockerfile":
				case "args", "target":
					target[key] = value
				default:
					return fmt.Errorf("unsupported build option %s for %s", key, name)
				}
			}
			_, backend := within(filepath.Join(repo, "back"), textAt(build, "context"))
			if backend {
				if revision != "" {
					if _, ok := secrets.Services[name]; !ok {
						return fmt.Errorf("BACKEND_CONFIG.services is missing %s", name)
					}
					args := mapAt(target, "args")
					args["CONFIG_REVISION"] = revision
					target["args"] = args
					target["secret"] = []string{"type=env,id=backend_config,env=BACKEND_CONFIG"}
					// Secret-bearing build layers must never be exported to PR-readable caches.
				}
				delete(service, "environment")
				// Public JWT/CA PEMs are embedded too; no host key-file dependency.
				var volumes []any
				for _, item := range listAt(service, "volumes") {
					mount := item.(map[string]any)
					if target := textAt(mount, "target"); target == "/secrets" || strings.HasPrefix(target, "/secrets/") {
						continue
					}
					volumes = append(volumes, item)
				}
				if len(volumes) == 0 {
					delete(service, "volumes")
				} else {
					service["volumes"] = volumes
				}
			} else {
				target["cache-from"] = []string{"type=gha,scope=sam-" + name}
				target["cache-to"] = []string{"type=gha,scope=sam-" + name + ",mode=max"}
			}
			delete(service, "build")
			service["image"] = tag
			targets[name] = target
			names, images = append(names, name), append(images, tag)
		}
		for _, item := range listAt(service, "volumes") {
			mount := item.(map[string]any)
			if textAt(mount, "type") != "bind" {
				continue
			}
			source := textAt(mount, "source")
			relative, inside := within(repo, source)
			if !inside {
				continue
			}
			stable := strings.HasPrefix(relative, "back/secrets/") ||
				strings.HasPrefix(relative, "back/deploy/postgres/") || relative == "back/deploy/seed"
			if stable {
				mount["source"] = filepath.Join(o.root, strings.TrimPrefix(relative, "back/"))
			} else {
				for _, part := range strings.Split(relative, "/") {
					if part == "secrets" || strings.HasPrefix(part, ".env") {
						return errors.New("secret runtime files must not be packaged")
					}
				}
				assets[relative] = true
				mount["source"] = filepath.Join(serverRelease, "assets", relative)
			}
			mount["bind"] = object{"create_host_path": false}
		}
	}
	if len(targets) == 0 {
		return errors.New("Compose contains no build services")
	}
	for asset := range assets {
		if err := copyPath(filepath.Join(repo, asset), filepath.Join(payload, "assets", asset)); err != nil {
			return err
		}
	}
	if revision != "" {
		if err := writeInfra(filepath.Join(payload, "infrastructure.env"), secrets.Infrastructure); err != nil {
			return err
		}
	}
	sort.Strings(names)
	sort.Strings(images)
	for path, value := range map[string]any{
		filepath.Join(payload, "compose.json"):  config,
		filepath.Join(o.output, "bake.json"):    object{"group": object{"default": object{"targets": names}}, "target": targets},
		filepath.Join(payload, "manifest.json"): manifest{Release: o.release, Root: o.root, Images: images},
	} {
		if err := writeJSON(path, value); err != nil {
			return err
		}
	}
	fmt.Printf("Prepared %d application images\n", len(images))
	return nil
}

func writeInfra(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var content strings.Builder
	for _, key := range keys {
		value := values[key]
		if !regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`).MatchString(key) || strings.ContainsAny(value, "\r\n") {
			return errors.New("infrastructure settings must have valid names and single-line values")
		}
		fmt.Fprintf(&content, "%s='%s'\n", key, strings.ReplaceAll(value, "'", `\'`))
	}
	return os.WriteFile(path, []byte(content.String()), 0o600)
}

func pack(o options) error {
	payload := filepath.Join(o.output, "payload")
	var m manifest
	if err := readJSON(filepath.Join(payload, "manifest.json"), &m); err != nil {
		return err
	}
	if len(m.Images) == 0 {
		return errors.New("release has no application images")
	}
	if _, err := command(false, "docker", append([]string{"image", "save", "-o", filepath.Join(payload, "images.tar")}, m.Images...)...); err != nil {
		return err
	}
	data, err := command(true, "docker", append([]string{"image", "inspect"}, m.Images...)...)
	if err != nil {
		return err
	}
	var images []struct {
		ID   string `json:"Id"`
		Size uint64
	}
	if json.Unmarshal([]byte(data), &images) != nil || len(images) != len(m.Images) {
		return errors.New("cannot inspect built images")
	}
	m.ImageIDs = map[string]string{}
	for index, image := range images {
		m.ImageBytes += image.Size
		m.ImageIDs[m.Images[index]] = image.ID
	}
	if err := writeJSON(filepath.Join(payload, "manifest.json"), m); err != nil {
		return err
	}
	return createTar(payload, filepath.Join(o.output, "release.tar"))
}

func createTar(source, destination string) error {
	file, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	writer := tar.NewWriter(file)
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil || path == source {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.IsDir() && !info.Mode().IsRegular() {
			return errors.New("archives cannot contain symlinks or devices")
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name, err = filepath.Rel(source, path)
		if err != nil {
			return err
		}
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(writer, in)
		return err
	})
	return errors.Join(err, writer.Close(), file.Close())
}

func extract(archive, destination string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	reader := tar.NewReader(file)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(header.Name) || (header.Typeflag != tar.TypeDir && header.Typeflag != tar.TypeReg) {
			return errors.New("unsafe release archive entry")
		}
		path := filepath.Join(destination, header.Name)
		if header.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, fs.FileMode(header.Mode)&0o777)
		if err != nil {
			return err
		}
		_, copyErr := io.Copy(out, reader)
		if err := errors.Join(copyErr, out.Close()); err != nil {
			return err
		}
	}
}

func digest(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func space(root string, required uint64) error {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(root, &stat); err != nil {
		return err
	}
	if uint64(stat.Bavail)*uint64(stat.Bsize) < required+reserve {
		return fmt.Errorf("insufficient disk space: need %d bytes including reserve", required+reserve)
	}
	return nil
}

func compose(root, release string, capture bool, args ...string) (string, error) {
	env := filepath.Join(release, "infrastructure.env")
	if !exists(env) {
		env = filepath.Join(root, ".env")
	}
	base := []string{"compose", "--parallel", "2", "-p", "sam", "--env-file", env,
		"--project-directory", root, "-f", filepath.Join(release, "compose.json")}
	return command(capture, "docker", append(base, args...)...)
}

func activate(root, release string) error {
	_, err := compose(root, release, false, "up", "-d", "--no-build", "--pull", "never",
		"--remove-orphans", "--wait", "--wait-timeout", "180")
	return err
}

func verify(root, release string) error {
	if _, err := compose(root, release, false, "ps", "--all"); err != nil {
		return err
	}
	var config object
	if err := readJSON(filepath.Join(release, "compose.json"), &config); err != nil {
		return err
	}
	if _, ok := mapAt(config, "services")["frontend"]; !ok {
		return nil
	}
	if err := checkURL("http://127.0.0.1:5173/"); err != nil {
		return err
	}
	data, err := compose(root, release, true, "config", "--format", "json")
	if err != nil {
		return err
	}
	if json.Unmarshal([]byte(data), &config) != nil {
		return errors.New("invalid runtime Compose configuration")
	}
	domain := textAt(mapAt(mapAt(mapAt(config, "services"), "nginx"), "environment"), "DOMAIN")
	if domain != "" {
		for _, path := range []string{"/", "/api/v1/docs"} {
			if err := checkURL("https://" + domain + path); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkURL(url string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	for attempt := range 6 {
		resp, err := client.Get(url)
		if err == nil {
			io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 400 {
				return nil
			}
		}
		if attempt < 5 {
			time.Sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("HTTP verification failed: %s", url)
}

func lock(root string, action func(string) error) error {
	cd := filepath.Join(root, ".cd")
	if err := os.MkdirAll(cd, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(cd, 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(cd, "deploy.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return action(cd)
}

func bootstrap(root, releases string) (string, error) {
	config, err := composeConfig(filepath.Join(root, "compose.yaml"), filepath.Join(root, "compose.server.yaml"))
	if err != nil {
		return "", err
	}
	ids, err := command(true, "docker", "ps", "-aq", "--filter", "label=com.docker.compose.project=sam")
	if err != nil || len(strings.Fields(ids)) == 0 {
		return "", errors.New("expected an existing running Compose project sam")
	}
	data, err := command(true, "docker", append([]string{"inspect"}, strings.Fields(ids)...)...)
	if err != nil {
		return "", err
	}
	var containers []struct {
		Image  string
		Config struct{ Labels map[string]string }
	}
	if json.Unmarshal([]byte(data), &containers) != nil {
		return "", errors.New("cannot snapshot running containers")
	}
	images := map[string]string{}
	for _, container := range containers {
		images[container.Config.Labels["com.docker.compose.service"]] = container.Image
	}
	for name, value := range mapAt(config, "services") {
		service := value.(map[string]any)
		if images[name] == "" {
			return "", errors.New("legacy stack is incomplete; cannot snapshot rollback state")
		}
		delete(service, "build")
		service["image"] = images[name]
	}
	name := fmt.Sprintf("bootstrap-%d", time.Now().UnixNano())
	path := filepath.Join(releases, name)
	if err := os.MkdirAll(path, 0o700); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(path, "compose.json"), config); err != nil {
		return "", err
	}
	if err := writeJSON(filepath.Join(path, "manifest.json"), manifest{Release: name, Images: []string{}, ImageIDs: images, Bootstrap: true}); err != nil {
		return "", err
	}
	if err := copyPath(filepath.Join(root, ".env"), filepath.Join(path, "infrastructure.env")); err != nil {
		return "", err
	}
	if err := os.Chmod(filepath.Join(path, "infrastructure.env"), 0o600); err != nil {
		return "", err
	}
	return name, os.WriteFile(filepath.Join(path, "success"), nil, 0o600)
}

func recoverPending(root, cd string, s state) error {
	marker := filepath.Join(cd, "pending.json")
	if !exists(marker) {
		return nil
	}
	fmt.Println("Restoring the committed release after an interrupted deployment")
	path := filepath.Join(cd, "releases", s.Current)
	if err := activate(root, path); err != nil {
		return err
	}
	if err := verify(root, path); err != nil {
		return err
	}
	return os.Remove(marker)
}

func cleanup(root string, s state) error {
	releases := filepath.Join(root, ".cd/releases")
	keep, protectedIDs := map[string]bool{}, map[string]bool{}
	for _, name := range []string{s.Current, s.Previous} {
		if name == "" {
			continue
		}
		var m manifest
		if !releasePattern.MatchString(name) {
			return errors.New("invalid retained release in state.json")
		}
		if err := readJSON(filepath.Join(releases, name, "manifest.json"), &m); err != nil {
			return err
		}
		for _, ref := range m.Images {
			keep[ref] = true
		}
		for _, id := range m.ImageIDs {
			protectedIDs[id] = true
		}
	}
	if _, err := command(false, "docker", "container", "prune", "--force", "--filter", "label=com.docker.compose.project=sam"); err != nil {
		return err
	}
	entries, err := os.ReadDir(releases)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == s.Current || entry.Name() == s.Previous {
			continue
		}
		path := filepath.Join(releases, entry.Name())
		var m manifest
		if readJSON(filepath.Join(path, "manifest.json"), &m) == nil {
			for _, ref := range m.Images {
				if strings.HasPrefix(ref, "sam/") && !keep[ref] {
					command(true, "docker", "image", "rm", ref) // no force: referenced images stay
				}
			}
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	refs, err := command(true, "docker", "image", "ls", "--filter", "label=io.sam.cd=true", "--format", "{{.Repository}}:{{.Tag}}")
	if err != nil {
		return err
	}
	for _, ref := range strings.Fields(refs) {
		if strings.HasPrefix(ref, "sam/") && !keep[ref] {
			command(true, "docker", "image", "rm", ref)
		}
	}
	ids, err := command(true, "docker", "image", "ls", "--quiet", "--no-trunc", "--filter", "dangling=true", "--filter", "label=io.sam.cd=true")
	if err != nil {
		return err
	}
	for _, id := range strings.Fields(ids) {
		if !protectedIDs[id] {
			command(true, "docker", "image", "rm", id)
		}
	}
	return nil
}

func preflight(o options) error {
	if o.archiveBytes == 0 {
		return errors.New("archive size must be positive")
	}
	return lock(o.root, func(cd string) error {
		var s state
		if exists(filepath.Join(cd, "state.json")) {
			if err := readJSON(filepath.Join(cd, "state.json"), &s); err != nil {
				return err
			}
			if err := recoverPending(o.root, cd, s); err != nil {
				return err
			}
			if err := cleanup(o.root, s); err != nil {
				return err
			}
		} else if !exists(filepath.Join(o.root, ".env")) {
			return errors.New("first deployment requires the existing server .env for rollback")
		}
		incoming := filepath.Join(cd, "incoming")
		entries, err := os.ReadDir(incoming)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, entry := range entries {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if entry.IsDir() && time.Since(info.ModTime()) > 24*time.Hour {
				if err := os.RemoveAll(filepath.Join(incoming, entry.Name())); err != nil {
					return err
				}
			}
		}
		if err := space(o.root, 3*o.archiveBytes); err != nil {
			return err
		}
		if _, err := command(false, "docker", "compose", "version"); err != nil {
			return err
		}
		architecture, err := command(true, "docker", "info", "--format", "{{.OSType}}/{{.Architecture}}")
		if err != nil {
			return err
		}
		if !slices.Contains([]string{"linux/x86_64", "linux/amd64"}, strings.TrimSpace(architecture)) {
			return errors.New("server must run Linux amd64")
		}
		return nil
	})
}

func deploy(o options) error {
	return lock(o.root, func(cd string) (result error) {
		releases := filepath.Join(cd, "releases")
		if err := os.MkdirAll(releases, 0o700); err != nil {
			return err
		}
		statePath := filepath.Join(cd, "state.json")
		if !exists(statePath) {
			baseline, err := bootstrap(o.root, releases)
			if err != nil {
				return err
			}
			if err := writeJSON(statePath, state{Current: baseline}); err != nil {
				return err
			}
		}
		var s state
		if err := readJSON(statePath, &s); err != nil {
			return err
		}
		if err := recoverPending(o.root, cd, s); err != nil {
			return err
		}
		if err := cleanup(o.root, s); err != nil {
			return err
		}
		path := filepath.Join(releases, o.release)
		if exists(path) {
			return errors.New("release ID already exists; rerun the GitHub job")
		}
		defer func() {
			os.Remove(o.archive)
			if !exists(filepath.Join(cd, "pending.json")) {
				result = errors.Join(result, cleanup(o.root, s))
			}
		}()
		actual, err := digest(o.archive)
		if err != nil {
			return err
		}
		if actual != o.checksum {
			return errors.New("archive checksum mismatch")
		}
		info, err := os.Stat(o.archive)
		if err != nil {
			return err
		}
		if err := space(o.root, 2*uint64(info.Size())); err != nil {
			return err
		}
		if err := extract(o.archive, path); err != nil {
			return err
		}
		var m manifest
		if err := readJSON(filepath.Join(path, "manifest.json"), &m); err != nil {
			return err
		}
		if m.Release != o.release || m.Root != o.root || !exists(filepath.Join(path, "infrastructure.env")) {
			return errors.New("release metadata/configuration does not match deployment")
		}
		if err := space(o.root, m.ImageBytes); err != nil {
			return err
		}
		if _, err := compose(o.root, path, true, "config", "--quiet"); err != nil {
			return err
		}
		if _, err := command(false, "docker", "image", "load", "--input", filepath.Join(path, "images.tar")); err != nil {
			return err
		}
		if err := os.Remove(filepath.Join(path, "images.tar")); err != nil {
			return err
		}
		for ref, expected := range m.ImageIDs {
			actual, err := command(true, "docker", "image", "inspect", "--format", "{{.Id}}", ref)
			if err != nil || strings.TrimSpace(actual) != expected {
				return errors.New("loaded image ID mismatch")
			}
		}
		images, err := compose(o.root, path, true, "config", "--images")
		if err != nil {
			return err
		}
		for _, image := range strings.Fields(images) {
			if _, err := command(true, "docker", "image", "inspect", "--format", "{{.Id}}", image); err != nil {
				return errors.New("missing infrastructure image; provision it before deployment")
			}
		}
		pending := filepath.Join(cd, "pending.json")
		if err := writeJSON(pending, object{"release": o.release}); err != nil {
			return err
		}
		err = activate(o.root, path)
		if err == nil {
			err = verify(o.root, path)
		}
		if err != nil {
			fmt.Println("Deployment failed; restoring the previous release (SQL migrations are not reversed)")
			compose(o.root, path, false, "ps", "--all")
			old := filepath.Join(releases, s.Current)
			if restoreErr := activate(o.root, old); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("rollback failed: %w", restoreErr))
			}
			if restoreErr := verify(o.root, old); restoreErr != nil {
				return errors.Join(err, fmt.Errorf("rollback verification failed: %w", restoreErr))
			}
			return errors.Join(err, os.Remove(pending))
		}
		if err := os.WriteFile(filepath.Join(path, "success"), nil, 0o600); err != nil {
			return err
		}
		s = state{Current: o.release, Previous: s.Current}
		if err := writeJSON(statePath, s); err != nil {
			return err
		}
		if err := os.Remove(pending); err != nil {
			return err
		}
		fmt.Println("Deployment successful:", o.release)
		return nil
	})
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func deliver() error {
	get := os.Getenv
	for _, key := range []string{"SSH_PRIVATE_KEY", "SSH_KNOWN_HOSTS", "DEPLOY_HOST", "DEPLOY_USER", "DEPLOY_PORT", "DEPLOY_ROOT", "RELEASE_ID", "RELEASE_ARCHIVE"} {
		if get(key) == "" {
			return fmt.Errorf("%s is required", key)
		}
	}
	root, id := get("DEPLOY_ROOT"), get("RELEASE_ID")
	if !rootPattern.MatchString(root) || root == "/" || filepath.Clean(root) != root || !releasePattern.MatchString(id) {
		return errors.New("invalid deployment root or release ID")
	}
	if !regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.-]*$`).MatchString(get("DEPLOY_HOST")) ||
		!regexp.MustCompile(`^[a-z_][a-z0-9_-]*$`).MatchString(get("DEPLOY_USER")) {
		return errors.New("invalid SSH host/user")
	}
	port, err := strconv.Atoi(get("DEPLOY_PORT"))
	if err != nil || port < 1 || port > 65535 {
		return errors.New("invalid SSH port")
	}
	dir, err := os.MkdirTemp("", "sam-ssh-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	for name, value := range map[string]string{"key": get("SSH_PRIVATE_KEY"), "known_hosts": get("SSH_KNOWN_HOSTS")} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o600); err != nil {
			return err
		}
	}
	os.Unsetenv("SSH_PRIVATE_KEY")
	os.Unsetenv("SSH_KNOWN_HOSTS")
	base := []string{"-i", filepath.Join(dir, "key"), "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
		"-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + filepath.Join(dir, "known_hosts"),
		"-o", "ConnectTimeout=15", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4"}
	target := get("DEPLOY_USER") + "@" + get("DEPLOY_HOST")
	remote := func(args ...string) error {
		quoted := make([]string, len(args))
		for index, arg := range args {
			quoted[index] = shellQuote(arg)
		}
		_, err := command(false, "ssh", append(slices.Clone(base), "-p", strconv.Itoa(port), target, strings.Join(quoted, " "))...)
		return err
	}
	upload := func(source, destination string) error {
		_, err := command(false, "scp", append(slices.Clone(base), "-P", strconv.Itoa(port), source, target+":"+destination)...)
		return err
	}
	incoming := filepath.Join(root, ".cd/incoming", id)
	if err := remote("mkdir", "-p", "-m", "700", incoming); err != nil {
		return err
	}
	defer remote("rm", "-rf", "--", incoming)
	tool, err := os.Executable()
	if err != nil {
		return err
	}
	remoteTool := filepath.Join(incoming, "release")
	if err := upload(tool, remoteTool); err != nil {
		return err
	}
	if err := remote("chmod", "700", remoteTool); err != nil {
		return err
	}
	archive := get("RELEASE_ARCHIVE")
	info, err := os.Stat(archive)
	if err != nil {
		return err
	}
	checksum, err := digest(archive)
	if err != nil {
		return err
	}
	if err := remote(remoteTool, "preflight", "--root", root, "--archive-bytes", strconv.FormatInt(info.Size(), 10)); err != nil {
		return err
	}
	remoteArchive := filepath.Join(incoming, "release.tar")
	if err := upload(archive, remoteArchive); err != nil {
		return err
	}
	return remote(remoteTool, "deploy", "--root", root, "--release", id, "--archive", remoteArchive, "--sha256", checksum)
}
