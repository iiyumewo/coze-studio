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
	"io"
	"path"
	"sort"
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
	ArchiveData []byte
}

type skillArchiveMember struct {
	file *zip.File
	name string
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

	members := make([]skillArchiveMember, 0, len(archive.File))
	var unpackedSize int64
	for _, file := range archive.File {
		isDir := file.FileInfo().IsDir() || strings.HasSuffix(file.Name, "/")
		name, err := normalizeSkillArchiveEntryName(file.Name, isDir)
		if err != nil {
			return nil, err
		}
		if isDir {
			continue
		}
		if file.UncompressedSize64 > uint64(maxSkillUnpackedSize) || unpackedSize > maxSkillUnpackedSize-int64(file.UncompressedSize64) {
			return nil, fmt.Errorf("skill archive uncompressed content is too large")
		}
		unpackedSize += int64(file.UncompressedSize64)
		members = append(members, skillArchiveMember{file: file, name: name})
	}
	manifestMember, err := selectSkillManifest(members)
	if err != nil {
		return nil, err
	}
	normalizedMembers, err := normalizeSkillArchiveMembers(members, manifestMember.name)
	if err != nil {
		return nil, err
	}
	if len(normalizedMembers) == 0 {
		return nil, fmt.Errorf("skill archive must contain SKILL.md at its root")
	}

	reader, err := manifestMember.file.Open()
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
	normalizedArchive, err := buildNormalizedSkillArchive(normalizedMembers)
	if err != nil {
		return nil, fmt.Errorf("normalize skill archive: %w", err)
	}
	if int64(len(normalizedArchive)) > maxSkillArchiveSize {
		return nil, fmt.Errorf("normalized skill archive exceeds %d MB", maxSkillArchiveSize/(1024*1024))
	}
	return &skillArchiveMetadata{
		Name:        manifest.Name,
		Description: manifest.Description,
		EntryCount:  int32(len(normalizedMembers)),
		ArchiveData: normalizedArchive,
	}, nil
}

func normalizeSkillArchiveEntryName(name string, isDir bool) (string, error) {
	originalName := name
	if isDir {
		name = strings.TrimSuffix(name, "/")
	}
	if name == "" || name == "." || name == ".." || strings.ContainsRune(name, '\x00') || strings.Contains(name, "\\") || path.IsAbs(name) || path.Clean(name) != name || strings.HasPrefix(name, "../") {
		return "", fmt.Errorf("skill archive contains an unsafe path: %q", originalName)
	}
	return name, nil
}

func selectSkillManifest(members []skillArchiveMember) (skillArchiveMember, error) {
	candidates := make([]skillArchiveMember, 0)
	for _, member := range members {
		if path.Base(member.name) == "SKILL.md" {
			candidates = append(candidates, member)
		}
	}
	if len(candidates) == 0 {
		return skillArchiveMember{}, fmt.Errorf("skill archive must contain SKILL.md at its root")
	}
	sort.Slice(candidates, func(i, j int) bool {
		leftParent, rightParent := path.Dir(candidates[i].name), path.Dir(candidates[j].name)
		leftDepth, rightDepth := skillArchivePathDepth(leftParent), skillArchivePathDepth(rightParent)
		if leftDepth != rightDepth {
			return leftDepth < rightDepth
		}
		return candidates[i].name < candidates[j].name
	})
	selectedParent := path.Dir(candidates[0].name)
	selectedDepth := skillArchivePathDepth(selectedParent)
	for _, candidate := range candidates[1:] {
		parent := path.Dir(candidate.name)
		if skillArchivePathDepth(parent) == selectedDepth && parent != selectedParent {
			return skillArchiveMember{}, fmt.Errorf("skill archive contains multiple Skill roots")
		}
	}
	return candidates[0], nil
}

func skillArchivePathDepth(parent string) int {
	if parent == "." || parent == "" {
		return 0
	}
	return strings.Count(parent, "/") + 1
}

func normalizeSkillArchiveMembers(members []skillArchiveMember, manifestName string) ([]skillArchiveMember, error) {
	prefix := path.Dir(manifestName)
	if prefix == "." {
		prefix = ""
	} else {
		prefix += "/"
	}
	allowOutsidePrefix := prefix != "" && !strings.Contains(strings.TrimSuffix(prefix, "/"), "/")
	normalized := make(map[string]skillArchiveMember, len(members))
	for _, member := range members {
		name := member.name
		if prefix != "" {
			if strings.HasPrefix(name, prefix) {
				name = strings.TrimPrefix(name, prefix)
			} else if allowOutsidePrefix {
				continue
			} else {
				return nil, fmt.Errorf("skill archive contains files outside skill root %q", strings.TrimSuffix(prefix, "/"))
			}
		}
		if name == "" || name == "." || name == ".." {
			return nil, fmt.Errorf("skill archive contains an invalid normalized path: %q", name)
		}
		if _, exists := normalized[name]; exists {
			return nil, fmt.Errorf("skill archive contains duplicate path: %q", name)
		}
		normalized[name] = skillArchiveMember{file: member.file, name: name}
	}
	result := make([]skillArchiveMember, 0, len(normalized))
	for _, member := range normalized {
		result = append(result, member)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })
	return result, nil
}

func buildNormalizedSkillArchive(members []skillArchiveMember) ([]byte, error) {
	var data bytes.Buffer
	writer := zip.NewWriter(&data)
	for _, member := range members {
		reader, err := member.file.Open()
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
		entryWriter, err := writer.Create(member.name)
		if err == nil {
			_, err = io.Copy(entryWriter, reader)
		}
		closeErr := reader.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = writer.Close()
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return data.Bytes(), nil
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
