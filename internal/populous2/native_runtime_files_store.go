package populous2

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// NativeRuntimeFileStore supplies real DOS operations inside an explicit
// configured save root. Names remain native byte strings; lookup ignores
// ASCII case, while directory enumeration retains the host stream order.
type NativeRuntimeFileStore struct {
	Root    string
	Volumes []string
	DOS     *NativeDOSFilesystem
}

func NewNativeRuntimeFileStore(root string, volumes []string, async bool) (*NativeRuntimeFileStore, error) {
	if root == "" {
		return nil, fmt.Errorf("native save root is not configured")
	}
	absolute, e := filepath.Abs(root)
	if e != nil {
		return nil, e
	}
	absolute, e = filepath.EvalSymlinks(absolute)
	if e != nil {
		return nil, e
	}
	info, e := os.Stat(absolute)
	if e != nil {
		return nil, e
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("native save root is not a directory")
	}
	s := &NativeRuntimeFileStore{Root: absolute, Volumes: append([]string(nil), volumes...)}
	s.DOS = NewNativeDOSFilesystem(absolute)
	s.DOS.Async = async
	s.DOS.ResolvePath = s.ResolvePath
	return s, nil
}
func (s *NativeRuntimeFileStore) ResolvePath(name []byte) (string, error) {
	if s == nil || s.Root == "" {
		return "", fmt.Errorf("native save root missing")
	}
	value := string(name)
	if strings.IndexByte(value, 0) >= 0 {
		return "", fmt.Errorf("native DOS path contains embedded NUL")
	}
	if i := strings.IndexByte(value, ':'); i >= 0 {
		volume := value[:i]
		allowed := false
		for _, candidate := range s.Volumes {
			if asciiDOSNameEqual(volume, candidate) {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("native save volume %s is not configured", volume)
		}
		value = value[i+1:]
	}
	if filepath.IsAbs(value) || strings.Contains(value, ":") {
		return "", fmt.Errorf("native save path is outside configured root")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("native save path escapes configured root")
	}
	current := s.Root
	if clean == "." {
		return current, nil
	}
	for _, part := range strings.Split(clean, string(filepath.Separator)) {
		candidate := filepath.Join(current, part)
		// Recover the actual directory spelling even on case-insensitive hosts.
		directory, e := os.Open(current)
		if e == nil {
			names, readErr := directory.Readdirnames(-1)
			_ = directory.Close()
			if readErr != nil {
				return "", readErr
			}
			for _, actual := range names {
				if asciiDOSNameEqual(actual, part) {
					candidate = filepath.Join(current, actual)
					break
				}
			}
		}
		if _, e := os.Lstat(candidate); e == nil {
			resolved, e := filepath.EvalSymlinks(candidate)
			if e != nil {
				return "", e
			}
			relative, e := filepath.Rel(s.Root, resolved)
			if e != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
				return "", fmt.Errorf("native save symlink leaves configured root")
			}
			current = resolved
		} else if os.IsNotExist(e) {
			current = candidate
		} else {
			return "", e
		}
	}
	return current, nil
}
func (s *NativeRuntimeFileStore) Port() NativeDOSPort {
	if s == nil || s.DOS == nil {
		return NativeDOSPort{}
	}
	return s.DOS.Port()
}
func (s *NativeRuntimeFileStore) Close() error {
	if s == nil || s.DOS == nil {
		return nil
	}
	return s.DOS.Close()
}

// Delete is an explicit host operation. The original $3f92 browser has no
// delete action; callers must request this operation separately.
func (s *NativeRuntimeFileStore) Delete(name []byte) error {
	path, e := s.ResolvePath(name)
	if e != nil {
		return e
	}
	if path == s.Root {
		return fmt.Errorf("native save root cannot be deleted")
	}
	info, e := os.Stat(path)
	if e != nil {
		return e
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("native save delete requires a regular file")
	}
	return os.Remove(path)
}
