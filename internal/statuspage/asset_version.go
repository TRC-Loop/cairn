// SPDX-License-Identifier: AGPL-3.0-or-later
package statuspage

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"sort"
	"sync"
)

var (
	assetVersionOnce sync.Once
	assetVersionVal  string
)

// AssetVersion returns a short hex digest derived from the contents of the
// embedded static tree. Stable across restarts for a given binary, changes
// whenever any embedded asset changes. Used to bust browser caches.
func AssetVersion() string {
	assetVersionOnce.Do(func() {
		h := sha256.New()
		root := StaticFS()
		var paths []string
		_ = fs.WalkDir(root, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			paths = append(paths, p)
			return nil
		})
		sort.Strings(paths)
		for _, p := range paths {
			b, err := fs.ReadFile(root, p)
			if err != nil {
				continue
			}
			h.Write([]byte(p))
			h.Write([]byte{0})
			h.Write(b)
		}
		assetVersionVal = hex.EncodeToString(h.Sum(nil))[:10]
	})
	return assetVersionVal
}
