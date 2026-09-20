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
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/coze-dev/coze-studio/backend/bizpkg/fileutil"
)

const (
	instructionsFileName    = "SKILL.md"
	maxInstructionsFileSize = 256 << 10
)

type Definition struct {
	ID           string
	Name         string
	Description  string
	RelativePath string
}

type Catalog struct {
	root   string
	skills []Definition
	byID   map[string]int
	byName map[string]int
}

func DefaultRoot() string {
	if root := strings.TrimSpace(os.Getenv("COZE_SKILLS_ROOT")); root != "" {
		return root
	}
	return filepath.Join(fileutil.GetWorkingDirectory(), "resources", "conf", "skills")
}

func DefaultWorkspaceRoot() string {
	if root := strings.TrimSpace(os.Getenv("COZE_SKILLS_WORKSPACE_ROOT")); root != "" {
		return root
	}
	return filepath.Join(fileutil.GetWorkingDirectory(), "runtime", "conversations")
}

func DefaultDependencyRoot() string {
	return filepath.Join(fileutil.GetWorkingDirectory(), "runtime", "skill-deps")
}

func DefaultRequirementsPath() string {
	return filepath.Join(fileutil.GetWorkingDirectory(), "resources", "conf", "skill-runtime", "python-requirements.txt")
}

func DefaultRuntimeRoot() string {
	if root := strings.TrimSpace(os.Getenv("COZE_SKILLS_RUNTIME_ROOT")); root != "" {
		return root
	}
	return filepath.Join(fileutil.GetWorkingDirectory(), "runtime", "skills")
}

func LoadCatalog(root string) (*Catalog, error) {
	return loadCatalog(root, nil)
}

// LoadCatalogForIDs loads only the selected Skill directories from a runtime root.
// This keeps stale caches for other Skills from becoming available to an Agent.
func LoadCatalogForIDs(root string, ids []string) (*Catalog, error) {
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	return loadCatalog(root, allowed)
}

func loadCatalog(root string, allowed map[string]struct{}) (*Catalog, error) {
	if strings.TrimSpace(root) == "" {
		root = DefaultRoot()
	}
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve skills root: %w", err)
	}

	catalog := &Catalog{
		root:   absoluteRoot,
		skills: make([]Definition, 0),
		byID:   make(map[string]int),
		byName: make(map[string]int),
	}
	entries, err := os.ReadDir(absoluteRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return catalog, nil
		}
		return nil, fmt.Errorf("read skills root: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if allowed != nil {
			if _, ok := allowed[entry.Name()]; !ok {
				continue
			}
		}
		definition, err := loadDefinition(absoluteRoot, entry.Name())
		if err != nil {
			return nil, err
		}
		if definition == nil {
			continue
		}
		if _, exists := catalog.byID[definition.ID]; exists {
			return nil, fmt.Errorf("duplicate skill ID %q", definition.ID)
		}
		nameKey := strings.ToLower(definition.Name)
		if _, exists := catalog.byName[nameKey]; exists {
			return nil, fmt.Errorf("duplicate skill name %q", definition.Name)
		}
		catalog.byID[definition.ID] = len(catalog.skills)
		catalog.byName[nameKey] = len(catalog.skills)
		catalog.skills = append(catalog.skills, *definition)
	}

	sort.Slice(catalog.skills, func(i, j int) bool {
		return catalog.skills[i].ID < catalog.skills[j].ID
	})
	catalog.rebuildIndexes()
	return catalog, nil
}

func (c *Catalog) Root() string {
	if c == nil {
		return ""
	}
	return c.root
}

func (c *Catalog) Skills() []Definition {
	if c == nil {
		return nil
	}
	result := make([]Definition, len(c.skills))
	copy(result, c.skills)
	return result
}

func (c *Catalog) Find(identifier string) (Definition, bool) {
	if c == nil {
		return Definition{}, false
	}
	identifier = strings.TrimSpace(identifier)
	if index, ok := c.byID[identifier]; ok {
		return c.skills[index], true
	}
	if index, ok := c.byName[strings.ToLower(strings.TrimSpace(identifier))]; ok {
		return c.skills[index], true
	}
	return Definition{}, false
}

func (c *Catalog) ReadInstructions(identifier string) (string, error) {
	definition, ok := c.Find(identifier)
	if !ok {
		return "", fmt.Errorf("skill %q was not found", identifier)
	}
	path := filepath.Join(c.root, definition.RelativePath, instructionsFileName)
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat skill instructions: %w", err)
	}
	if info.Size() > maxInstructionsFileSize {
		return "", fmt.Errorf("skill instructions exceed %d bytes", maxInstructionsFileSize)
	}
	if err = validatePathWithinRoot(c.root, path); err != nil {
		return "", err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read skill instructions: %w", err)
	}
	return string(content), nil
}

func (c *Catalog) ResolveScriptPath(identifier, scriptPath string) (string, error) {
	definition, ok := c.Find(identifier)
	if !ok {
		return "", fmt.Errorf("skill %q was not found", identifier)
	}
	if strings.TrimSpace(scriptPath) == "" {
		return "", errors.New("script path is required")
	}
	if filepath.IsAbs(scriptPath) {
		return "", errors.New("script path must be relative to the skill directory")
	}
	cleanPath := filepath.Clean(scriptPath)
	if cleanPath == "." || cleanPath == ".." || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return "", errors.New("script path escapes the skill directory")
	}
	return filepath.Join(definition.RelativePath, cleanPath), nil
}

func (c *Catalog) PromptDescription() string {
	if c == nil || len(c.skills) == 0 {
		return "No local Skills are available."
	}
	var builder strings.Builder
	builder.WriteString("The following local Skills are available:\n")
	for _, definition := range c.skills {
		fmt.Fprintf(&builder, "- %s (id: %s): %s\n", definition.Name, definition.ID, definition.Description)
	}
	builder.WriteString("When a Skill is relevant, call skill_read with its id to load the full instructions before calling script_run.")
	return builder.String()
}

type frontMatter struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
}

func loadDefinition(root, id string) (*Definition, error) {
	path := filepath.Join(root, id, instructionsFileName)
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("stat skill %q instructions: %w", id, err)
	}
	if info.Size() > maxInstructionsFileSize {
		return nil, fmt.Errorf("skill %q instructions exceed %d bytes", id, maxInstructionsFileSize)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read skill %q instructions: %w", id, err)
	}
	metadata, body, err := parseFrontMatter(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse skill %q front matter: %w", id, err)
	}
	name := strings.TrimSpace(metadata.Name)
	if name == "" {
		name = id
	}
	description := compactText(metadata.Description)
	if description == "" {
		description = firstDescriptionLine(body)
	}
	if description == "" {
		description = "Local Skill"
	}
	return &Definition{ID: id, Name: name, Description: description, RelativePath: id}, nil
}

func parseFrontMatter(content string) (metadata frontMatter, body string, err error) {
	content = strings.TrimPrefix(content, "\ufeff")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return metadata, content, nil
	}
	end := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end < 0 {
		return metadata, "", errors.New("front matter closing marker is missing")
	}
	if err = yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &metadata); err != nil {
		return metadata, "", err
	}
	return metadata, strings.Join(lines[end+1:], "\n"), nil
}

func firstDescriptionLine(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = compactText(strings.TrimLeft(line, "#"))
		if line != "" {
			return line
		}
	}
	return ""
}

func compactText(value string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
}

func validatePathWithinRoot(root, target string) error {
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return fmt.Errorf("resolve skills root: %w", err)
	}
	targetReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		return fmt.Errorf("resolve skill path: %w", err)
	}
	rel, err := filepath.Rel(rootReal, targetReal)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return errors.New("skill path resolves outside the skills root")
	}
	return nil
}

func (c *Catalog) rebuildIndexes() {
	for key := range c.byID {
		delete(c.byID, key)
	}
	for key := range c.byName {
		delete(c.byName, key)
	}
	for index, definition := range c.skills {
		c.byID[definition.ID] = index
		c.byName[strings.ToLower(definition.Name)] = index
	}
}
