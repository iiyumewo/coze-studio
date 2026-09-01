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
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

const (
	maxSkillArchiveSize       int64 = 20 * 1024 * 1024
	maxSkillUnpackedSize      int64 = 100 * 1024 * 1024
	maxSkillArchiveEntries          = 1000
	maxSkillNameLength              = 128
	maxSkillDescriptionLength       = 2000
)

type skillManifest struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

type skillArchiveMetadata struct {
	Name        string
	Description string
	EntryCount  int32
}

func validateSkillArchive(fileName string, data []byte) (*skillArchiveMetadata, error) {
	ext := strings.ToLower(path.Ext(fileName))
	if ext != ".zip" && ext != ".skill" {
		return nil, fmt.Errorf("skill archive must use .zip or .skill extension")
	}
	if int64(len(data)) == 0 || int64(len(data)) > maxSkillArchiveSize {
		return nil, fmt.Errorf("skill archive must be between 1 byte and %d MB", maxSkillArchiveSize/(1024*1024))
	}

	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid skill archive: %w", err)
	}
	if len(archive.File) == 0 {
		return nil, fmt.Errorf("skill archive is empty")
	}
	if len(archive.File) > maxSkillArchiveEntries {
		return nil, fmt.Errorf("skill archive contains too many entries")
	}

	var manifestFile *zip.File
	var unpackedSize int64
	for _, file := range archive.File {
		if file.Name == "" || path.IsAbs(file.Name) || path.Clean(file.Name) != file.Name || strings.HasPrefix(file.Name, "../") {
			return nil, fmt.Errorf("skill archive contains an unsafe path: %q", file.Name)
		}
		if file.UncompressedSize64 > uint64(maxSkillUnpackedSize) || unpackedSize > maxSkillUnpackedSize-int64(file.UncompressedSize64) {
			return nil, fmt.Errorf("skill archive uncompressed content is too large")
		}
		unpackedSize += int64(file.UncompressedSize64)
		if file.Name == "SKILL.md" && !file.FileInfo().IsDir() {
			manifestFile = file
		}
	}
	if manifestFile == nil {
		return nil, fmt.Errorf("skill archive must contain SKILL.md at its root")
	}

	reader, err := manifestFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open SKILL.md: %w", err)
	}
	var manifestBytes bytes.Buffer
	_, copyErr := manifestBytes.ReadFrom(reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", copyErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close SKILL.md: %w", closeErr)
	}

	manifest, err := parseSkillManifest(manifestBytes.String())
	if err != nil {
		return nil, err
	}
	return &skillArchiveMetadata{
		Name:        manifest.Name,
		Description: manifest.Description,
		EntryCount:  int32(len(archive.File)),
	}, nil
}

func parseSkillManifest(content string) (*skillManifest, error) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, fmt.Errorf("SKILL.md must start with YAML frontmatter")
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return nil, fmt.Errorf("SKILL.md YAML frontmatter is not closed")
	}

	manifest := &skillManifest{}
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), manifest); err != nil {
		return nil, fmt.Errorf("invalid SKILL.md YAML frontmatter: %w", err)
	}
	manifest.Name = strings.TrimSpace(manifest.Name)
	manifest.Description = strings.TrimSpace(manifest.Description)
	if manifest.Name == "" || utf8.RuneCountInString(manifest.Name) > maxSkillNameLength {
		return nil, fmt.Errorf("SKILL.md name is required and must be at most %d characters", maxSkillNameLength)
	}
	if manifest.Description == "" || utf8.RuneCountInString(manifest.Description) > maxSkillDescriptionLength {
		return nil, fmt.Errorf("SKILL.md description is required and must be at most %d characters", maxSkillDescriptionLength)
	}
	return manifest, nil
}
