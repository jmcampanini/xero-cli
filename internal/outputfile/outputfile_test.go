package outputfile

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestWritePublishesOrPreservesDestination(t *testing.T) {
	for _, scenario := range []string{"new", "existing", "overwrite", "symlink", "replace symlink", "directory", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "records.json")
			original := []byte("original")
			switch scenario {
			case "existing", "overwrite", "canceled":
				if err := os.WriteFile(path, original, 0o600); err != nil {
					t.Fatal(err)
				}
			case "symlink", "replace symlink":
				target := filepath.Join(dir, "target")
				if err := os.WriteFile(target, original, 0o600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, path); err != nil {
					t.Fatal(err)
				}
			case "directory":
				if err := os.Mkdir(path, 0o700); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			overwrite := scenario == "overwrite" || scenario == "replace symlink" || scenario == "directory" || scenario == "canceled"
			body := []byte("complete payload\n")

			err := Write(ctx, path, body, overwrite)

			success := scenario == "new" || scenario == "overwrite" || scenario == "replace symlink"
			if (err == nil) != success {
				t.Fatalf("Write = %v, want success=%v", err, success)
			}
			if scenario == "directory" {
				if info, err := os.Stat(path); err != nil || !info.IsDir() {
					t.Fatalf("directory changed: %v", err)
				}
			} else {
				got, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				want := original
				if success {
					want = body
				}
				if !bytes.Equal(got, want) {
					t.Errorf("destination = %q, want %q", got, want)
				}
			}
			if scenario == "symlink" || scenario == "replace symlink" {
				got, err := os.ReadFile(filepath.Join(dir, "target"))
				if err != nil || !bytes.Equal(got, original) {
					t.Errorf("symlink target = %q, %v, want original", got, err)
				}
			}
			leftovers, err := filepath.Glob(filepath.Join(dir, ".xero-output-*"))
			if err != nil || len(leftovers) != 0 {
				t.Errorf("temporary files = %v, %v", leftovers, err)
			}
			if success {
				info, err := os.Stat(path)
				if err != nil || info.Mode().Perm() != 0o600 {
					t.Errorf("output permissions = %v, %v, want 0600", info, err)
				}
			}
		})
	}
}

func TestWriteConcurrentCreationDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "records.json")
	bodies := [][]byte{bytes.Repeat([]byte("a"), 65536), bytes.Repeat([]byte("b"), 65536)}
	start := make(chan struct{})
	errorsCh := make(chan error, len(bodies))
	for _, body := range bodies {
		go func() {
			<-start
			errorsCh <- Write(context.Background(), path, body, false)
		}()
	}
	close(start)
	successes := 0
	for range bodies {
		if err := <-errorsCh; err == nil {
			successes++
		} else if !errors.Is(err, os.ErrExist) {
			t.Errorf("Write = %v, want existing file error", err)
		}
	}
	if successes != 1 {
		t.Fatalf("successful writes = %d, want 1", successes)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, bodies[0]) && !bytes.Equal(got, bodies[1]) {
		t.Errorf("destination = %d bytes, %v, want one complete payload", len(got), err)
	}
}
