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

package impl

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/coze-dev/coze-studio/backend/types/consts"
)

func TestNewFromEnvRejectsUnknownRunnerType(t *testing.T) {
	t.Setenv("COZE_SKILL_RUNNER_TYPE", "unknown")
	_, err := NewFromEnv(t.TempDir())
	require.ErrorContains(t, err, "unsupported skill runner type")
}

func TestNewFromEnvCreatesLocalRunner(t *testing.T) {
	t.Setenv("COZE_SKILL_RUNNER_TYPE", RunnerTypeLocal)
	runner, err := NewFromEnv(t.TempDir())
	require.NoError(t, err)
	require.NotNil(t, runner)
}

func TestConfigFromEnvDefaultsToTsinghuaMirror(t *testing.T) {
	t.Setenv("PIP_INDEX_URL", "https://pypi.org/simple")
	t.Setenv(consts.SkillRunnerPipMirror, "")

	config := configFromEnv(t.TempDir())
	require.Equal(t, defaultPipMirror, config.PipMirrorURL)
}

func TestConfigFromEnvAllowsExplicitMirror(t *testing.T) {
	t.Setenv(consts.SkillRunnerPipMirror, "https://mirror.example.com/simple")

	config := configFromEnv(t.TempDir())
	require.Equal(t, "https://mirror.example.com/simple", config.PipMirrorURL)
}
