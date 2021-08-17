package fastzip

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"os"
	"path/filepath"
	"strings"
)

type FileSanityFn func(name string) (string, error)

var (
	ErrSkipFile = fmt.Errorf("skip file")
)

// ExtractEase extracts files, but not creates symlinks and directories
// from the archive
// also no mod sync, metadata sync
func (e *Extractor) ExtractEase(ctx context.Context, fileSanityFn FileSanityFn) (err error) {
	limiter := make(chan struct{}, e.options.concurrency)

	wg, ctx := errgroup.WithContext(ctx)
	defer func() {
		if werr := wg.Wait(); werr != nil {
			err = werr
		}
	}()

	for i, file := range e.zr.File {
		if file.Mode()&irregularModes != 0 {
			continue
		}

		var (
			name string
			path string
		)
		name, err = fileSanityFn(file.Name)
		if err == ErrSkipFile {
			continue
		} else if err != nil {
			return err
		}
		path, err = filepath.Abs(filepath.Join(e.chroot, name))
		if err != nil {
			return err
		}

		if !strings.HasPrefix(path, e.chroot) {
			return fmt.Errorf("%s cannot be extracted outside of chroot (%s)", path, e.chroot)
		}

		if err := os.MkdirAll(filepath.Dir(path), 0777); err != nil {
			return err
		}

		if ctx.Err() != nil {
			return ctx.Err()
		}

		switch {
		case file.Mode()&os.ModeSymlink != 0:
			// ignore
			continue

		case file.Mode().IsDir():
			// ignore

		default:
			limiter <- struct{}{}

			gf := e.zr.File[i]
			wg.Go(func() error {
				defer func() { <-limiter }()
				return e.createFile(ctx, path, gf)
			})
		}
		if err != nil {
			return err
		}
	}

	if err := wg.Wait(); err != nil {
		return err
	}

	return nil
}
