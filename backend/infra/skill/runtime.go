/*
 * Copyright 2025 coze-dev Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package skill

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/coze-dev/coze-studio/backend/infra/storage"
)

const (
	maxRuntimeArchiveSize  int64 = 20 << 20
	maxRuntimeUnpackedSize int64 = 100 << 20
	maxRuntimeEntries            = 1000
)

// RuntimeSkill describes an uploaded Skill archive that can be materialized locally.
type RuntimeSkill struct {
	ID          int64
	SpaceID     int64
	Name        string
	Description string
	ObjectKey   string
}

var runtimeMaterializeLocks sync.Map

// PrepareRuntimeCatalog downloads and safely extracts the selected Skills when their
// workspace-local cache is missing, then returns a catalog containing only those Skills.
func PrepareRuntimeCatalog(ctx context.Context, objectStorage storage.Storage, root string, spaceID int64, skills []RuntimeSkill) (*Catalog, error) {
	if spaceID <= 0 {
		return nil, errors.New("space ID must be positive")
	}
	if strings.TrimSpace(root) == "" {
		root = DefaultRuntimeRoot()
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve runtime skills root: %w", err)
	}
	if len(skills) == 0 {
		return LoadCatalogForIDs(filepath.Join(absoluteRoot, strconv.FormatInt(spaceID, 10)), nil)
	}
	if objectStorage == nil {
		return nil, errors.New("skill object storage is not initialized")
	}

	spaceRoot := filepath.Join(absoluteRoot, strconv.FormatInt(spaceID, 10))
	if err = os.MkdirAll(spaceRoot, 0o750); err != nil {
		return nil, fmt.Errorf("create runtime skills root: %w", err)
	}
	ids := make([]string, 0, len(skills))
	for _, runtimeSkill := range skills {
		if runtimeSkill.ID <= 0 || runtimeSkill.SpaceID != spaceID {
			return nil, fmt.Errorf("invalid runtime Skill %d", runtimeSkill.ID)
		}
		if strings.TrimSpace(runtimeSkill.ObjectKey) == "" {
			return nil, fmt.Errorf("runtime Skill %d has no archive object", runtimeSkill.ID)
		}
		id := strconv.FormatInt(runtimeSkill.ID, 10)
		ids = append(ids, id)
		if err = materializeRuntimeSkill(ctx, objectStorage, spaceRoot, runtimeSkill); err != nil {
			return nil, err
		}
	}

	return LoadCatalogForIDs(spaceRoot, ids)
}

func materializeRuntimeSkill(ctx context.Context, objectStorage storage.Storage, spaceRoot string, runtimeSkill RuntimeSkill) error {
	id := strconv.FormatInt(runtimeSkill.ID, 10)
	target := filepath.Join(spaceRoot, id)
	lockValue, _ := runtimeMaterializeLocks.LoadOrStore(target, &sync.Mutex{})
	lock := lockValue.(*sync.Mutex)
	lock.Lock()
	defer lock.Unlock()

	if isMaterializedSkill(target) {
		return nil
	}
	if _, err := os.Stat(target); err == nil {
		if err = os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove incomplete Skill %d cache: %w", runtimeSkill.ID, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect Skill %d cache: %w", runtimeSkill.ID, err)
	}

	archive, err := objectStorage.GetObject(ctx, runtimeSkill.ObjectKey)
	if err != nil {
		return fmt.Errorf("download Skill %d archive: %w", runtimeSkill.ID, err)
	}
	if int64(len(archive)) == 0 || int64(len(archive)) > maxRuntimeArchiveSize {
		return fmt.Errorf("Skill %d archive exceeds the runtime size limit", runtimeSkill.ID)
	}

	temp := target + ".tmp-" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if err = extractRuntimeArchive(archive, temp); err != nil {
		_ = os.RemoveAll(temp)
		return fmt.Errorf("extract Skill %d archive: %w", runtimeSkill.ID, err)
	}
	if err = os.Rename(temp, target); err != nil {
		_ = os.RemoveAll(temp)
		return fmt.Errorf("publish Skill %d cache: %w", runtimeSkill.ID, err)
	}
	return nil
}

func isMaterializedSkill(directory string) bool {
	directoryInfo, err := os.Lstat(directory)
	if err != nil || !directoryInfo.IsDir() || directoryInfo.Mode()&os.ModeSymlink != 0 {
		return false
	}
	instructionsInfo, err := os.Lstat(filepath.Join(directory, instructionsFileName))
	return err == nil && !instructionsInfo.IsDir() && instructionsInfo.Mode()&os.ModeSymlink == 0
}

func extractRuntimeArchive(data []byte, target string) error {
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid zip archive: %w", err)
	}
	if len(archive.File) == 0 || len(archive.File) > maxRuntimeEntries {
		return errors.New("Skill archive has an invalid entry count")
	}
	if err = os.MkdirAll(target, 0o755); err != nil {
		return err
	}

	var declaredSize int64
	var extractedSize int64
	for _, entry := range archive.File {
		isDir := entry.FileInfo().IsDir() || strings.HasSuffix(entry.Name, "/")
		entryName, err := normalizeRuntimeArchiveEntryName(entry.Name, isDir)
		if err != nil {
			return err
		}
		if entry.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symbolic links are not allowed: %q", entry.Name)
		}
		if entry.UncompressedSize64 > uint64(maxRuntimeUnpackedSize) || declaredSize > maxRuntimeUnpackedSize-int64(entry.UncompressedSize64) {
			return errors.New("Skill archive is too large after extraction")
		}
		declaredSize += int64(entry.UncompressedSize64)

		destination := filepath.Join(target, filepath.FromSlash(entryName))
		if err = validateArchiveDestination(target, destination); err != nil {
			return err
		}
		if isDir {
			if err = os.MkdirAll(destination, 0o755); err != nil {
				return err
			}
			continue
		}
		if err = os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
			return err
		}
		reader, openErr := entry.Open()
		if openErr != nil {
			return openErr
		}
		file, createErr := os.OpenFile(destination, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if createErr == nil {
			remaining := maxRuntimeUnpackedSize - extractedSize
			var written int64
			written, createErr = io.Copy(file, io.LimitReader(reader, remaining+1))
			if createErr == nil && written > remaining {
				createErr = errors.New("Skill archive is too large after extraction")
			}
			extractedSize += written
		}
		closeFileErr := fileClose(file)
		closeReaderErr := reader.Close()
		if createErr != nil {
			return createErr
		}
		if closeFileErr != nil {
			return closeFileErr
		}
		if closeReaderErr != nil {
			return closeReaderErr
		}
	}
	if !isMaterializedSkill(target) {
		return errors.New("Skill archive must contain SKILL.md at its root")
	}
	return nil
}

func normalizeRuntimeArchiveEntryName(name string, isDir bool) (string, error) {
	originalName := name
	if isDir {
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("unsafe archive path %q", originalName)
	}
	return name, nil
}

func validateArchiveDestination(root, destination string) error {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(destination))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("archive path resolves outside the extraction directory")
	}
	return nil
}

func fileClose(file *os.File) error {
	if file == nil {
		return nil
	}
	return file.Close()
}
