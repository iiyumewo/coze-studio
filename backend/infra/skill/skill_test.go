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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadCatalog(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "calculator"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "calculator", instructionsFileName), []byte(`---
name: Calculator
description: Add two numbers.
---

# Calculator

Use calculator.py with two numeric parameters.
`), 0o640))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "calculator", "references"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "calculator", "references", "rules.md"), []byte("# Rules\n\nRead this reference.\n"), 0o640))
	require.NoError(t, os.MkdirAll(filepath.Join(root, "plain"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "plain", instructionsFileName), []byte("# Plain text skill\n\nRead this file first.\n"), 0o640))
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".hidden"), 0o750))

	catalog, err := LoadCatalog(root)
	require.NoError(t, err)
	require.Equal(t, root, catalog.Root())
	require.Len(t, catalog.Skills(), 2)

	calculator, ok := catalog.Find("calculator")
	require.True(t, ok)
	require.Equal(t, "Calculator", calculator.Name)
	require.Equal(t, "Add two numbers.", calculator.Description)

	plain, ok := catalog.Find("plain")
	require.True(t, ok)
	require.Equal(t, "plain", plain.ID)
	require.Equal(t, "Plain text skill", plain.Description)
	require.Contains(t, catalog.PromptDescription(), "Calculator")
	require.Contains(t, catalog.PromptDescription(), "Add two numbers.")

	instructions, err := catalog.ReadInstructions("calculator")
	require.NoError(t, err)
	require.Contains(t, instructions, "calculator.py")

	content, err := catalog.ReadFile("calculator", "references/rules.md")
	require.NoError(t, err)
	require.Contains(t, content, "Read this reference")

	path, err := catalog.ResolveScriptPath("calculator", "scripts/calculator.py")
	require.NoError(t, err)
	require.Equal(t, filepath.Join("calculator", "scripts", "calculator.py"), path)
	_, err = catalog.ResolveScriptPath("calculator", "../outside.py")
	require.ErrorContains(t, err, "escapes the skill directory")
	_, err = catalog.ResolveScriptPath("calculator", "references/rules.md")
	require.ErrorContains(t, err, "must have a .py extension")
	_, err = catalog.ReadFile("calculator", "../outside.md")
	require.ErrorContains(t, err, "escapes the skill directory")
}

func TestLoadCatalogMissingRootReturnsEmptyCatalog(t *testing.T) {
	catalog, err := LoadCatalog(filepath.Join(t.TempDir(), "missing"))
	require.NoError(t, err)
	require.Empty(t, catalog.Skills())
	require.Equal(t, "No local Skills are available.", catalog.PromptDescription())
}
