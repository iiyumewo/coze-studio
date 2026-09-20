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

package scriptrunner

import "context"

type Config struct {
	SkillsRoot               string
	WorkspaceRoot            string
	DependencyRoot           string
	RequirementsPath         string
	PipMirrorURL             string
	BuildNetwork             string
	DependencyVersionsToKeep int
	PythonPath               string
	TimeoutSeconds           int
	MaxOutputBytes           int

	DockerBinary        string
	DockerImage         string
	DockerMemoryLimitMB int64
	DockerCPUs          float64
	DockerPidsLimit     int64
}

type RunRequest struct {
	ScriptPath     string
	ConversationID int64
	Params         map[string]any
}

type RunResponse struct {
	Stdout     string
	Stderr     string
	ExitCode   int
	WorkingDir string
}

//go:generate mockgen -destination ../../internal/mock/infra/scriptrunner/script_mock.go --package scriptrunner -source script.go
type Runner interface {
	Run(ctx context.Context, request *RunRequest) (*RunResponse, error)
}
