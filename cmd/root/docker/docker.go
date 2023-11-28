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

// docker package provides access to log collections on D4R
package docker

import (
	"fmt"
	"strings"

	"github.com/dremio/dremio-diagnostic-collector/cmd/root/cli"
)

type DockerArgs struct {
	DockerPath       	 string
}

// NewDockerExecActions is the only supported way to initialize the DockerExecActions struct
func NewDockerExecActions(dockerArgs DockerArgs) *DockerExecActions {
	return &DockerExecActions{
		cli:                  &cli.Cli{},
		dockerPath:       	  dockerArgs.DockerPath,
	}
}

// DockerExecActions provides a way to collect and copy files using docker exec
type DockerExecActions struct {
	cli                  cli.CmdExecutor
	dockerPath           string
}



func (c *DockerExecActions) HostExecuteAndStream(mask bool,hostString string, output cli.OutputHandler, isCoordinator bool, args ...string) (err error) {
	dockerExecArgs := []string{c.dockerPath, "exec", hostString}
	dockerExecArgs = append(dockerExecArgs, args...)
	return c.cli.ExecuteAndStreamOutput(mask, output, dockerExecArgs...)
}


func (c *DockerExecActions) HostExecute(mask bool,hostString string,isCoordinator bool, args ...string) (out string, err error) {
	dockerExecArgs := []string{c.dockerPath, "exec", hostString}
	dockerExecArgs = append(dockerExecArgs, args...)
	return c.cli.Execute(mask, dockerExecArgs...)
}

// Host = Container
func (c *DockerExecActions) CopyFromHost(hostString string,isCoordinator bool, source, destination string) (out string, err error) {
	return c.cli.Execute(false, c.dockerPath, "cp", fmt.Sprintf("%v:%v", hostString, source), destination)
}


func (c *DockerExecActions) CopyFromHostSudo(hostString string, isCoordinator bool, _, source, destination string) (out string, err error) {

	// We dont have any sudo user in the container so no addition of sudo commands used
	return c.CopyFromHost("",isCoordinator,source,destination);
}

func (c *DockerExecActions) CopyToHost(hostString string, isCoordinator bool, source, destination string) (out string, err error) {

		
	out, err = c.cli.Execute(false, c.dockerPath, "cp",source, fmt.Sprintf("%v:%v", hostString, destination))
	if err != nil {
		return out, err
	}
	return  c.cli.Execute(false,c.dockerPath,"exec","--user","root",hostString,"chown","dremio:dremio",destination)
}

func (c *DockerExecActions) CopyToHostSudo(hostString string, isCoordinator bool, _, source, destination string) (out string, err error) {

	// We dont have any sudo user in the container so no addition of sudo commands used
	return c.CopyToHost(hostString,isCoordinator,source,destination);
}

func (c *DockerExecActions) FindHosts(searchTerm string) (containerName []string, err error) {
	out, err := c.cli.Execute(false, c.dockerPath, "ps", "--filter", "name="+ searchTerm, "--format", "'{{.Names}}'")
	if err != nil {
		return []string{}, err
	}
	rawContainers := strings.Split(out, "\n")
	// check for executor necessary, because name of coordinator was localdremioincontainer an executor was localdremioincontainer-executor, 
	//so when you serach for coordinators you also got the executor
	executor := strings.Contains(searchTerm,"executor");
	var containers []string
	for _, container := range rawContainers {
		container = strings.Trim(container,"'");		
		if container == ""  || (strings.Contains(container,"executor") != executor)   {
			continue
		}
		containers = append(containers, container)
	}
	return containers, nil
	
}

func (c *DockerExecActions) HelpText() string {
	return "Make sure the labels and namespace you use actually correspond to your dremio containers: try something like 'ddc --mode docker --docker-path docker --coordinator localdremioincontainer --executors localdremioincontainer-executor'.  You can also run 'docker ps' to see what containers are available to use for your dremio containers";
}
