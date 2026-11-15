package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func useComponent(t *testing.T, name string) {
	t.Helper()
	previous := component
	component = name
	t.Cleanup(func() { component = previous })
}

func TestComponentIsolation(t *testing.T) {
	for _, name := range []string{"backend", "frontend"} {
		t.Run(name, func(t *testing.T) {
			useComponent(t, name)
			config := componentConfig(object{"services": object{"frontend": object{}, "license": object{}, "postgres": object{}}})
			services := mapAt(config, "services")
			_, frontend := services["frontend"]
			_, license := services["license"]
			if frontend != (name == "frontend") || license != (name == "backend") {
				t.Fatalf("wrong services for %s: %v", name, services)
			}
			for ref, owner := range map[string]string{"sam/frontend:v1": "frontend", "sam/license:v1": "backend", "sam/worker:v1": "backend", "postgres:17": ""} {
				if ownsImage(ref) != (name == owner) {
					t.Fatalf("wrong ownership of %s for %s", ref, name)
				}
			}
			want := "/opt/sam/.cd"
			if name == "frontend" {
				want += "-frontend"
			}
			if got := cdRoot("/opt/sam"); got != want {
				t.Fatalf("state root: %s", got)
			}
		})
	}
}

func TestLegacyRollbackDoesNotActivateOtherComponent(t *testing.T) {
	for _, name := range []string{"backend", "frontend"} {
		t.Run(name, func(t *testing.T) {
			useComponent(t, name)
			dir := t.TempDir()
			// Record the actual Docker invocation without touching running containers.
			docker := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOCKER_ARGS\"\n"
			if err := os.WriteFile(filepath.Join(dir, "docker"), []byte(docker), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DOCKER_ARGS", filepath.Join(dir, "args"))
			original := object{"services": object{"frontend": object{"image": "sam/frontend:old"}, "license": object{"image": "sam/license:old"}}}
			if err := writeJSON(filepath.Join(dir, "compose.json"), original); err != nil {
				t.Fatal(err)
			}
			if err := activate(dir, dir); err != nil {
				t.Fatal(err)
			}
			args, err := os.ReadFile(filepath.Join(dir, "args"))
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(args), "--remove-orphans") {
				t.Fatal("activation may remove the other component")
			}
			var scoped object
			if err := readJSON(filepath.Join(dir, "compose."+name+".json"), &scoped); err != nil {
				t.Fatal(err)
			}
			if len(mapAt(scoped, "services")) != 1 {
				t.Fatalf("unscoped rollback: %v", scoped)
			}
			var unchanged object
			if err := readJSON(filepath.Join(dir, "compose.json"), &unchanged); err != nil {
				t.Fatal(err)
			}
			if len(mapAt(unchanged, "services")) != 2 {
				t.Fatal("legacy release was modified")
			}
		})
	}
}

func TestCleanupDoesNotRemoveOtherComponentsImages(t *testing.T) {
	for _, name := range []string{"backend", "frontend"} {
		t.Run(name, func(t *testing.T) {
			useComponent(t, name)
			root := t.TempDir()
			bin := t.TempDir()
			log := filepath.Join(bin, "removed")
			docker := `#!/bin/sh
if [ "$1 $2" = "image ls" ]; then
 case " $* " in
  *dangling=true*) exit 0 ;;
 esac
 printf '%s\n' sam/frontend:keep sam/frontend:old sam/license:keep sam/license:old
elif [ "$1 $2" = "image rm" ]; then
 printf '%s\n' "$3" >> "$REMOVAL_LOG"
else
 exit 1
fi
`
			if err := os.WriteFile(filepath.Join(bin, "docker"), []byte(docker), 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("REMOVAL_LOG", log)
			release := filepath.Join(cdRoot(root), "releases", "current")
			if err := os.MkdirAll(release, 0o700); err != nil {
				t.Fatal(err)
			}
			// A legacy manifest may contain both components.
			if err := writeJSON(filepath.Join(release, "manifest.json"), manifest{Images: []string{"sam/frontend:keep", "sam/license:keep"}}); err != nil {
				t.Fatal(err)
			}
			if err := cleanup(root, state{Current: "current"}); err != nil {
				t.Fatal(err)
			}
			removed, err := os.ReadFile(log)
			if err != nil {
				t.Fatal(err)
			}
			want := "sam/license:old\n"
			if name == "frontend" {
				want = "sam/frontend:old\n"
			}
			if string(removed) != want {
				t.Fatalf("unexpected removed images: %q", removed)
			}
		})
	}
}
