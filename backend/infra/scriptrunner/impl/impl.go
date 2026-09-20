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
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/coze-dev/coze-studio/backend/infra/scriptrunner"
	dockerrunner "github.com/coze-dev/coze-studio/backend/infra/scriptrunner/impl/docker"
	localrunner "github.com/coze-dev/coze-studio/backend/infra/scriptrunner/impl/local"
	"github.com/coze-dev/coze-studio/backend/infra/skill"
	"github.com/coze-dev/coze-studio/backend/pkg/envkey"
	"github.com/coze-dev/coze-studio/backend/types/consts"
)

const (
	RunnerTypeLocal  = "local"
	RunnerTypeDocker = "docker"
	defaultPipMirror = "https://pypi.tuna.tsinghua.edu.cn/simple"
)

func NewFromEnv(skillsRoot string) (scriptrunner.Runner, error) {
	runnerType := strings.ToLower(strings.TrimSpace(envkey.GetStringD(consts.SkillRunnerType, RunnerTypeDocker)))
	config := configFromEnv(skillsRoot)

	switch runnerType {
	case RunnerTypeLocal:
		return localrunner.NewRunner(config)
	case RunnerTypeDocker:
		return dockerrunner.NewRunner(config)
	default:
		return nil, fmt.Errorf("unsupported skill runner type %q", runnerType)
	}
}

func PrepareDependenciesFromEnv(ctx context.Context) error {
	runnerType := strings.ToLower(strings.TrimSpace(envkey.GetStringD(consts.SkillRunnerType, RunnerTypeDocker)))
	if runnerType != RunnerTypeDocker {
		return nil
	}
	return dockerrunner.PrepareDependencies(ctx, configFromEnv(skill.DefaultRoot()))
}

func configFromEnv(skillsRoot string) scriptrunner.Config {
	return scriptrunner.Config{
		SkillsRoot:               skillsRoot,
		WorkspaceRoot:            envkey.GetString(consts.SkillRunnerWorkspaceRoot),
		DependencyRoot:           envkey.GetStringD(consts.SkillRunnerDependencyRoot, skill.DefaultDependencyRoot()),
		RequirementsPath:         envkey.GetStringD(consts.SkillRunnerRequirements, skill.DefaultRequirementsPath()),
		PipMirrorURL:             envkey.GetStringD(consts.SkillRunnerPipMirror, defaultPipMirror),
		BuildNetwork:             envkey.GetStringD(consts.SkillRunnerBuildNetwork, "bridge"),
		DependencyVersionsToKeep: envkey.GetIntD(consts.SkillRunnerDependencyVersionsToKeep, 2),
		TimeoutSeconds:           envkey.GetIntD(consts.SkillRunnerTimeoutSeconds, 60),
		MaxOutputBytes:           envkey.GetIntD(consts.SkillRunnerMaxOutputBytes, 1<<20),
		DockerBinary:             envkey.GetStringD(consts.SkillRunnerDockerBinary, "docker"),
		DockerImage:              envkey.GetStringD(consts.SkillRunnerDockerImage, "python:3.12-slim-bookworm"),
		DockerMemoryLimitMB:      int64(envkey.GetIntD(consts.SkillRunnerDockerMemoryMB, 256)),
		DockerCPUs:               getFloatD(consts.SkillRunnerDockerCPUs, 0.5),
		DockerPidsLimit:          int64(envkey.GetIntD(consts.SkillRunnerDockerPidsLimit, 64)),
	}
}

func getFloatD(key string, defaultValue float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return defaultValue
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultValue
	}
	return parsed
}
