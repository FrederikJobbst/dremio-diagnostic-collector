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

// docker package provides access to log collections on docker deployed environments
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
	d := DockerExecActions{
		cli:                  cli,
		dockerPath:          "docker",
	}
	out, err := d.HostExecute(false, "localdremioincontainer",false, "ls", "-l")
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
	expectedCall := []string{"docker", "exec", "localdremioincontainer", "ls", "-l"}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}


func TestDockerSearch(t *testing.T) {
	labelName := "executor"
	cli := &tests.MockCli{
		StoredResponse: []string{"localdremioincontainer-executor1\nlocaldremioincontainer-executor2\n"},
		StoredErrors:   []error{nil},
	}
	d := DockerExecActions{
		cli:         cli,
		dockerPath: "docker",
	}
	containerNames, err := d.FindHosts(labelName)
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}

	expectedContainers := []string{"localdremioincontainer-executor1","localdremioincontainer-executor2"};
	if !reflect.DeepEqual(containerNames, expectedContainers) {
		t.Errorf("expected %v call but got %v", expectedContainers, containerNames)
	}
	calls := cli.Calls
	if len(calls) != 1 {
		t.Errorf("expected 1 call but got %v", len(calls))
	}
	expectedCall := []string{"docker", "ps", "--filter", "name="+ labelName, "--format", "'{{.Names}}'"}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}

func TestDockerCopyFrom(t *testing.T) {
	containerName := "localdremioincontainer"
	source := filepath.Join(string(filepath.Separator), "containerroot", "test.log")
	destination := filepath.Join(string(filepath.Separator), "mydir", "test.log")
	cli := &tests.MockCli{
		StoredResponse: []string{"success"},
		StoredErrors:   []error{nil},
	}
	d := DockerExecActions{
		cli:                  cli,
		dockerPath:          "docker",
	}
	out, err := d.CopyFromHost(containerName, false, source, destination)
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
	expectedCall := []string{"docker", "cp", fmt.Sprintf("%v:%v", containerName, source), destination}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}

func TestDockerCopyFromWindowsHost(t *testing.T) {
	containerName := "localdremioincontainer"
	source := filepath.Join("containerroot", "test.log")
	destination := filepath.Join("C:", string(filepath.Separator), "mydir", "test.log")
	cli := &tests.MockCli{
		StoredResponse: []string{"success"},
		StoredErrors:   []error{nil},
	}
	d := DockerExecActions{
		cli:                  cli,
		dockerPath:          "docker",
	}
	out, err := d.CopyFromHost(containerName, false, source, destination)
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

	expectedDestination := filepath.Join("C:",string(filepath.Separator),"mydir", "test.log")
	expectedCall := []string{"docker", "cp", fmt.Sprintf("%v:%v", containerName, source), expectedDestination}
	if !reflect.DeepEqual(calls[0], expectedCall) {
		t.Errorf("\nexpected call\n%v\nbut got\n%v", expectedCall, calls[0])
	}
}



