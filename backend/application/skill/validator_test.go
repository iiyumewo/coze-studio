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
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateSkillArchiveAcceptsOfficialDirectoryLayout(t *testing.T) {
	data := testArchiveWithEntries(t, []testArchiveEntry{
		{name: "workspace-snapshot/", directory: true},
		{name: "workspace-snapshot/scripts/", directory: true},
		{name: "workspace-snapshot/references/", directory: true},
		{name: "workspace-snapshot/SKILL.md", content: "---\nname: workspace-snapshot\ndescription: test skill\n---\n"},
		{name: "workspace-snapshot/scripts/run.py", content: "print('ok')"},
		{name: "workspace-snapshot/references/guide.md", content: "# Guide\n"},
	})

	metadata, err := validateSkillArchive("workspace-snapshot.zip", data)
	require.NoError(t, err)
	require.Equal(t, "workspace-snapshot", metadata.Name)
	require.Equal(t, "test skill", metadata.Description)
	require.Equal(t, int32(3), metadata.EntryCount)

	normalized, err := zip.NewReader(bytes.NewReader(metadata.ArchiveData), int64(len(metadata.ArchiveData)))
	require.NoError(t, err)
	require.Equal(t, []string{"SKILL.md", "references/guide.md", "scripts/run.py"}, archiveFileNames(normalized.File))
}

func TestValidateSkillArchiveRejectsUnsafeDirectoryPath(t *testing.T) {
	data := testArchiveWithEntries(t, []testArchiveEntry{
		{name: "../", directory: true},
		{name: "SKILL.md", content: "---\nname: demo\ndescription: test\n---\n"},
	})

	_, err := validateSkillArchive("demo.zip", data)
	require.ErrorContains(t, err, "unsafe path")
}

func TestValidateSkillArchiveRejectsMultipleSkillRoots(t *testing.T) {
	data := testArchiveWithEntries(t, []testArchiveEntry{
		{name: "first/SKILL.md", content: "---\nname: first\ndescription: first\n---\n"},
		{name: "second/SKILL.md", content: "---\nname: second\ndescription: second\n---\n"},
	})

	_, err := validateSkillArchive("demo.zip", data)
	require.ErrorContains(t, err, "multiple Skill roots")
}

type testArchiveEntry struct {
	name      string
	content   string
	directory bool
}

func testArchiveWithEntries(t *testing.T, entries []testArchiveEntry) []byte {
	t.Helper()
	var data bytes.Buffer
	archive := zip.NewWriter(&data)
	for _, entry := range entries {
		header := &zip.FileHeader{Name: entry.name}
		if entry.directory {
			header.SetMode(os.ModeDir | 0o755)
		} else {
			header.SetMode(0o644)
		}
		writer, err := archive.CreateHeader(header)
		require.NoError(t, err)
		if !entry.directory {
			_, err = io.WriteString(writer, entry.content)
			require.NoError(t, err)
		}
	}
	require.NoError(t, archive.Close())
	return data.Bytes()
}

func archiveFileNames(files []*zip.File) []string {
	names := make([]string, 0, len(files))
	for _, file := range files {
		names = append(names, file.Name)
	}
	return names
}
