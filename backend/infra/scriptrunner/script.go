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

import (
	"context"
	"encoding/json"
	"fmt"
)

// ArgumentMode controls how a script receives its input.
//
// JSON is the original script_run contract: the script receives one JSON
// object at sys.argv[1]. Argv is for conventional command-line scripts and
// passes Args unchanged after the script path.
type ArgumentMode string

const (
	ArgumentModeJSON ArgumentMode = "json"
	ArgumentModeArgv ArgumentMode = "argv"
)

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
	Mode           ArgumentMode
	Params         map[string]any
	Args           []string
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

// BuildArguments converts a run request into the arguments appended after the
// Python script path. An empty mode preserves the original JSON behavior.
func BuildArguments(request *RunRequest) ([]string, error) {
	if request == nil {
		return nil, fmt.Errorf("script run request is nil")
	}

	switch request.Mode {
	case "", ArgumentModeJSON:
		if len(request.Args) > 0 {
			return nil, fmt.Errorf("args are only supported in %q mode", ArgumentModeArgv)
		}
		params := request.Params
		if params == nil {
			params = map[string]any{}
		}
		paramsJSON, err := json.Marshal(params)
		if err != nil {
			return nil, fmt.Errorf("marshal script params: %w", err)
		}
		return []string{string(paramsJSON)}, nil
	case ArgumentModeArgv:
		if len(request.Params) > 0 {
			return nil, fmt.Errorf("params are only supported in %q mode", ArgumentModeJSON)
		}
		return append([]string(nil), request.Args...), nil
	default:
		return nil, fmt.Errorf("unsupported script argument mode %q", request.Mode)
	}
}
