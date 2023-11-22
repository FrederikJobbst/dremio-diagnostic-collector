//	Copyright 2023 Dremio Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//  Copyright 2023 Dremio Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// kubernetes package provides access to log collections on k8s
package docker

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/dremio/dremio-diagnostic-collector/pkg/tests"
)


func TestDockerExec(t *testing.T) {

	cli := &tests.MockCli{
		StoredResponse: []string{"success"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:          "docker",
	}
	out, err := k.HostExecute(false, "",false, "ls", "-l")
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if out != "success" {
		t.Errorf("expected success but got %v", out)
	}
	calls := cli.Calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call but got %v", len(calls))
	}
	expectedCall := []string{"docker", "exec", "dremio-executor", "--", "ls", "-l"}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}


func TestDockerSearch(t *testing.T) {
	namespace := "testns"
	labelName := "myPods"
	cli := &tests.MockCli{
		StoredResponse: []string{"pod/pod1\npod/pod2\npod/pod3\n"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:         cli,
		dockerPath: "kubectl",
	}
	podNames, err := k.FindHosts(labelName)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	//we need to pass the namespace for later commands that may consume this
	// period is handy because it is an illegal character in a kubernetes name and so
	// can act as a separator
	expectedPods := []string{"pod1", "pod2", "pod3"}
	if !reflect.DeepEqual(podNames, expectedPods) {
		t.Errorf("expected %v call but got %v", expectedPods, podNames)
	}
	calls := cli.Calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call but got %v", len(calls))
	}
	expectedCall := []string{"kubectl", "get", "pods", "-n", namespace, "-l", labelName, "-o", "name"}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}

func TestDockerCopyFrom(t *testing.T) {
	namespace := "testns"
	podName := "pod"
	source := filepath.Join(string(filepath.Separator), "podroot", "test.log")
	destination := filepath.Join(string(filepath.Separator), "mydir", "test.log")
	cli := &tests.MockCli{
		StoredResponse: []string{"success"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:          "kubectl",
	}
	out, err := k.CopyFromHost(podName, false, source, destination)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if out != "success" {
		t.Errorf("expected success but got %v", out)
	}
	calls := cli.Calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call but got %v", len(calls))
	}
	expectedCall := []string{"kubectl", "cp", "-n", namespace, "-c", "dremio-executor", fmt.Sprintf("%v:%v", podName, source), destination}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}

func TestKubectCopyFromWindowsHost(t *testing.T) {
	namespace := "testns"
	podName := "pod"
	source := filepath.Join("podroot", "test.log")
	destination := filepath.Join("C:", string(filepath.Separator), "mydir", "test.log")
	cli := &tests.MockCli{
		StoredResponse: []string{"success"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:          "kubectl",
	}
	out, err := k.CopyFromHost(podName, false, source, destination)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	if out != "success" {
		t.Errorf("expected success but got %v", out)
	}
	calls := cli.Calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call but got %v", len(calls))
	}
	//we remove the C: due to issue found in https://github.com/kubernetes/kubernetes/issues/77310"
	expectedDestination := filepath.Join(string(filepath.Separator), "mydir", "test.log")
	expectedCall := []string{"kubectl", "cp", "-n", namespace, "-c", "dremio-executor", fmt.Sprintf("%v:%v", podName, source), expectedDestination}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}


/*
func TestNewKubectlK8sActions(t *testing.T) {
	kubectlPath := "kubectlPath"
	coordinatorContainer := "main"
	executorContainer := "exec"
	actions := NewDockerExecActions(DockerArgs{
		DockerPath:          kubectlPath,
	})

	if actions.coordinatorContainer != coordinatorContainer {
		t.Errorf("\nexpected \n%v\nbut got\n%v", coordinatorContainer, actions.coordinatorContainer)
	}

	if actions.executorContainer != executorContainer {
		t.Errorf("\nexpected \n%v\nbut got\n%v", executorContainer, actions.executorContainer)
	}
}


func TestGetContainerNameWhenIsMaster(t *testing.T) {
	cli := &tests.MockCli{
		StoredResponse: []string{"dremio-master-coordinator dremio-coordinator"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:       	  "docker",
		coordinatorContainer: "dremio-master-coordinator",
		executorContainer:    "dremio-executor",
	}
	containerName := k.getContainerName(true)
	if containerName != k.coordinatorContainer {
		t.Errorf("\nexpected \n%v\nbut got\n%v", k.coordinatorContainer, containerName)
	}
}

func TestGetContainerNameWhenIsCoordinator(t *testing.T) {
	cli := &tests.MockCli{
		StoredResponse: []string{"dremio-master-coordintor dremio-coordinator"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:           "docker",
		coordinatorContainer: "dremio-coordinator",
		executorContainer:    "dremio-executor",
	}
	containerName := k.getContainerName(true)
	if containerName != k.coordinatorContainer {
		t.Errorf("\nexpected \n%v\nbut got\n%v", k.coordinatorContainer, containerName)
	}
}

func TestGetContainerNameWhenIsExecutor(t *testing.T) {
	cli := &tests.MockCli{
		StoredResponse: []string{"dremio-executor"},
		StoredErrors:   []error{nil},
	}
	k := DockerExecActions{
		cli:                  cli,
		dockerPath:           "docker",
		coordinatorContainer: "dremio-coordinator",
		executorContainer:    "dremio-executor",
	}
	containerName := k.getContainerName(false)
	if containerName != k.executorContainer {
		t.Errorf("\nexpected \n%v\nbut got\n%v", k.executorContainer, containerName)
	}
}
*/