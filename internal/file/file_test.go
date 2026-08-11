package file

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPackUnpackPreservesFileMode verifies that file permission bits — in
// particular the executable bit — survive a pack → unpack round trip. The
// receiver must chmod from the tar header, since os.Create always drops the
// exec bit (mode 0666 & umask).
func TestPackUnpackPreservesFileMode(t *testing.T) {
	srcDir := t.TempDir()

	scriptPath := filepath.Join(srcDir, "run.sh")
	require.NoError(t, os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hi\n"), 0644))
	require.NoError(t, os.Chmod(scriptPath, 0755))

	docPath := filepath.Join(srcDir, "readme.txt")
	require.NoError(t, os.WriteFile(docPath, []byte("docs"), 0644))
	require.NoError(t, os.Chmod(docPath, 0600))

	subDir := filepath.Join(srcDir, "subdir")
	require.NoError(t, os.Mkdir(subDir, 0750))
	innerPath := filepath.Join(subDir, "inner.sh")
	require.NoError(t, os.WriteFile(innerPath, []byte("#!/bin/sh\n"), 0644))
	require.NoError(t, os.Chmod(innerPath, 0700))

	files, err := ReadFiles([]string{scriptPath, docPath, subDir})
	require.NoError(t, err)
	defer func() {
		for _, f := range files {
			_ = f.Close()
		}
	}()

	tarFile, _, err := PackFiles(files)
	require.NoError(t, err)

	dstDir := t.TempDir()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dstDir))
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
		_ = tarFile.Close()
	})

	unpacker, err := NewUnpacker(false, tarFile)
	require.NoError(t, err)
	defer func() { _ = unpacker.Close() }()

	for {
		committer, err := unpacker.Unpack()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "unpacking")
		_, err = committer.Commit()
		require.NoError(t, err, "committing %s", committer.FileName())
	}

	for _, tc := range []struct {
		name string
		want os.FileMode
	}{
		{"run.sh", 0755},
		{"readme.txt", 0600},
		{"subdir", 0750},
		{"subdir/inner.sh", 0700},
	} {
		got, err := os.Stat(filepath.Join(dstDir, tc.name))
		require.NoError(t, err, tc.name)
		assert.Equal(t, tc.want, got.Mode().Perm(), "perm bits for %s", tc.name)
	}
}
