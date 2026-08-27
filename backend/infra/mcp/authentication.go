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

package mcp

import (
	"encoding/json"
	"errors"
	"os"

	pluginencrypt "github.com/coze-dev/coze-studio/backend/domain/plugin/encrypt"
)

type HeaderAuthenticationPayload struct {
	HeaderName  string `json:"header_name"`
	HeaderValue string `json:"header_value"`
}

func EncryptAuthentication(authentication Authentication) (*string, error) {
	switch authentication.Type {
	case AuthenticationTypeNone:
		return nil, nil
	case AuthenticationTypeHeader:
		_, normalizedAuthentication, err := validateHeaderAuthentication("", authentication)
		if err != nil {
			return nil, err
		}
		authentication = normalizedAuthentication
	default:
		return nil, errors.New("unsupported MCP authentication type")
	}

	payload, err := json.Marshal(&HeaderAuthenticationPayload{
		HeaderName:  authentication.HeaderName,
		HeaderValue: authentication.HeaderValue,
	})
	if err != nil {
		return nil, err
	}

	encryptedAuthentication, err := pluginencrypt.EncryptByAES(payload, authenticationSecret())
	if err != nil {
		return nil, err
	}

	return &encryptedAuthentication, nil
}

func DecryptAuthentication(authenticationType AuthenticationType, encryptedAuthentication *string) (Authentication, error) {
	if authenticationType == AuthenticationTypeNone {
		return Authentication{Type: AuthenticationTypeNone}, nil
	}
	if authenticationType != AuthenticationTypeHeader {
		return Authentication{}, errors.New("unsupported stored MCP authentication type")
	}
	if encryptedAuthentication == nil || *encryptedAuthentication == "" {
		return Authentication{}, errors.New("encrypted MCP authentication is required")
	}

	payload, err := pluginencrypt.DecryptByAES(*encryptedAuthentication, authenticationSecret())
	if err != nil {
		return Authentication{}, err
	}

	var headerAuthentication HeaderAuthenticationPayload
	if err = json.Unmarshal(payload, &headerAuthentication); err != nil {
		return Authentication{}, err
	}

	_, authentication, err := validateHeaderAuthentication("", Authentication{
		Type:        AuthenticationTypeHeader,
		HeaderName:  headerAuthentication.HeaderName,
		HeaderValue: headerAuthentication.HeaderValue,
	})
	if err != nil {
		return Authentication{}, err
	}

	return authentication, nil
}

func authenticationSecret() string {
	secret := os.Getenv(pluginencrypt.AuthSecretEnv)
	if secret == "" {
		secret = pluginencrypt.DefaultAuthSecret
	}

	return secret
}
