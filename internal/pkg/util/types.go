/**
 * Copyright (C) 2015 Red Hat, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *         http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package util

import (
	"encoding/json"

	"github.com/pkg/errors"

	api "k8s.io/kubernetes/pkg/api/unversioned"
	client "k8s.io/kubernetes/pkg/client/unversioned"
)

type MasterType string

const (
	OpenShift  MasterType = "OpenShift"
	Kubernetes MasterType = "Kubernetes"
)

func TypeOfMaster(c *client.Client) (MasterType, error) {
	res, err := c.Get().AbsPath("").DoRaw()
	if err != nil {
		return "", errors.Wrap(err, "could not discover the type of your installation")
	}

	var rp api.RootPaths
	err = json.Unmarshal(res, &rp)
	if err != nil {
		errors.Wrap(err, "could not discover the type of your installation")
	}
	for _, p := range rp.Paths {
		if p == "/oapi" {
			return OpenShift, nil
		}
	}
	return Kubernetes, nil
}
