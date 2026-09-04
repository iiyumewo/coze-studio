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
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	storageMock "github.com/coze-dev/coze-studio/backend/internal/mock/infra/storage"
	"go.uber.org/mock/gomock"
)

func TestPrepareRuntimeCatalogReusesMaterializedSkill(t *testing.T) {
	archive := testSkillArchive(t)
	root := t.TempDir()
	ctrl := gomock.NewController(t)
	objectStorage := storageMock.NewMockStorage(ctrl)
	objectStorage.EXPECT().GetObject(gomock.Any(), "skills/42/7/demo.zip").Return(archive, nil).Times(1)

	definition := RuntimeSkill{ID: 7, SpaceID: 42, ObjectKey: "skills/42/7/demo.zip"}
	first, err := PrepareRuntimeCatalog(context.Background(), objectStorage, root, 42, []RuntimeSkill{definition})
	require.NoError(t, err)
	second, err := PrepareRuntimeCatalog(context.Background(), objectStorage, root, 42, []RuntimeSkill{definition})
	require.NoError(t, err)

	firstSkill, ok := first.Find("7")
	require.True(t, ok)
	secondSkill, ok := second.Find("7")
	require.True(t, ok)
	require.Equal(t, "7", firstSkill.ID)
	require.Equal(t, secondSkill.RelativePath, "7")
	require.FileExists(t, filepath.Join(root, "42", "7", instructionsFileName))
}

func TestPrepareRuntimeCatalogDoesNotExposeUnselectedCache(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "42", "8"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "42", "8", instructionsFileName), []byte("---\nname: stale\ndescription: stale\n---\n"), 0o640))

	catalog, err := PrepareRuntimeCatalog(context.Background(), nil, root, 42, nil)
	require.NoError(t, err)
	_, ok := catalog.Find("8")
	require.False(t, ok)
}

func TestExtractRuntimeArchiveRejectsUnsafePath(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	file, err := archive.Create("../escape.txt")
	require.NoError(t, err)
	_, err = file.Write([]byte("unsafe"))
	require.NoError(t, err)
	require.NoError(t, archive.Close())

	err = extractRuntimeArchive(data.Bytes(), filepath.Join(t.TempDir(), "skill"))
	require.ErrorContains(t, err, "unsafe archive path")
}

func TestExtractRuntimeArchiveAcceptsDirectoryEntries(t *testing.T) {
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	directory := &zip.FileHeader{Name: "scripts/"}
	directory.SetMode(os.ModeDir | 0o755)
	_, err := archive.CreateHeader(directory)
	require.NoError(t, err)
	manifest, err := archive.Create(instructionsFileName)
	require.NoError(t, err)
	_, err = io.WriteString(manifest, "---\nname: demo\ndescription: test skill\n---\n")
	require.NoError(t, err)
	script, err := archive.Create("scripts/main.py")
	require.NoError(t, err)
	_, err = io.WriteString(script, "print('ok')")
	require.NoError(t, err)
	require.NoError(t, archive.Close())

	target := filepath.Join(t.TempDir(), "skill")
	require.NoError(t, extractRuntimeArchive(data.Bytes(), target))
	require.FileExists(t, filepath.Join(target, instructionsFileName))
	require.FileExists(t, filepath.Join(target, "scripts", "main.py"))
}

func TestPrepareRuntimeCatalogSerializesConcurrentMaterialization(t *testing.T) {
	archive := testSkillArchive(t)
	root := t.TempDir()
	ctrl := gomock.NewController(t)
	objectStorage := storageMock.NewMockStorage(ctrl)
	objectStorage.EXPECT().GetObject(gomock.Any(), "skills/42/7/demo.zip").Return(archive, nil).Times(1)

	definition := RuntimeSkill{ID: 7, SpaceID: 42, ObjectKey: "skills/42/7/demo.zip"}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			catalog, err := PrepareRuntimeCatalog(context.Background(), objectStorage, root, 42, []RuntimeSkill{definition})
			if err != nil {
				errs <- err
				return
			}
			_, ok := catalog.Find("7")
			if !ok {
				errs <- errors.New("materialized Skill was not found in catalog")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
}

func testSkillArchive(t *testing.T) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	manifest, err := archive.Create(instructionsFileName)
	require.NoError(t, err)
	_, err = io.WriteString(manifest, "---\nname: demo\ndescription: test skill\n---\n")
	require.NoError(t, err)
	script, err := archive.Create("scripts/main.py")
	require.NoError(t, err)
	_, err = io.WriteString(script, "print('ok')")
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	return data.Bytes()
}
