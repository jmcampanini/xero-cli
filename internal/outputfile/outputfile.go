// Package outputfile publishes complete payloads without exposing partial files.
package outputfile

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Write atomically publishes body at path, refusing existing paths unless overwrite is true.
// Any error leaves the destination unchanged.
func Write(ctx context.Context, path string, body []byte, overwrite bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".xero-output-*")
	if err != nil {
		return fmt.Errorf("create temporary output beside %q: %w", path, err)
	}
	temporary := file.Name()
	defer func() { _ = os.Remove(temporary) }()

	_, writeErr := file.Write(body)
	closeErr := file.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return fmt.Errorf("write output %q: %w", path, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	if overwrite {
		err = os.Rename(temporary, path)
	} else {
		err = os.Link(temporary, path)
	}
	if err != nil {
		return fmt.Errorf("publish output %q: %w", path, err)
	}
	return nil
}
