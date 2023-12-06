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

// cmd package contains all the command line flag and initialization logic for commands
package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dremio/dremio-diagnostic-collector/cmd/awselogs"
	local "github.com/dremio/dremio-diagnostic-collector/cmd/local"
	"github.com/dremio/dremio-diagnostic-collector/cmd/local/conf"
	"github.com/dremio/dremio-diagnostic-collector/cmd/root/collection"
	"github.com/dremio/dremio-diagnostic-collector/cmd/root/docker"
	"github.com/dremio/dremio-diagnostic-collector/cmd/root/helpers"
	"github.com/dremio/dremio-diagnostic-collector/cmd/root/kubernetes"
	"github.com/dremio/dremio-diagnostic-collector/cmd/root/ssh"
	version "github.com/dremio/dremio-diagnostic-collector/cmd/version"
	"github.com/dremio/dremio-diagnostic-collector/pkg/consoleprint"
	"github.com/dremio/dremio-diagnostic-collector/pkg/masking"
	"github.com/dremio/dremio-diagnostic-collector/pkg/simplelog"
	"github.com/dremio/dremio-diagnostic-collector/pkg/versions"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// var scaleoutCoordinatorContainer string
var coordinatorContainer string
var executorsContainer string
var coordinatorStr string
var executorsStr string
var sshKeyLoc string
var sshUser string
var promptForDremioPAT bool
var transferDir string
var ddcYamlLoc string

var outputLoc string

var kubectlPath string
var sudoUser string
var namespace string

var dockerPath string

type usedMode int

const (
	undefined = 0
	SSH usedMode = 1
	K8S = 2
	D4R = 3
)

var (
	usedModeMap = map[string]usedMode{
		"kubernetes": K8S,
		"k":K8S,
		"docker": D4R,
		"d": D4R,
		"ssh": SSH,
		"s": SSH,
	}
)

func ParseString(str string) (usedMode) {
    c := usedModeMap[strings.ToLower(str)]
    return c
}

var modeUsedForCollecting string

// var isEmbeddedK8s bool
// var isEmbeddedSSH bool

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "ddc",
	Short: versions.GetCLIVersion() + " ddc connects via to dremio servers collects logs into an archive",
	Long: versions.GetCLIVersion() + ` ddc connects via ssh, docker or kubectl and collects a series of logs and files for dremio, then puts those collected files in an archive
examples:

for ssh based communication to VMs or Bare metal hardware:

	# coordinator only
	ddc --mode ssh --coordinator 10.0.0.19 --ssh-user myuser
	# coordinator and executors
	ddc --mode ssh --coordinator 10.0.0.19 --executors 10.0.0.20,10.0.0.21,10.0.0.22 --ssh-user myuser

for kubernetes deployments:

	# coordinator only
	ddc --mode kubernetes --namespace mynamespace --coordinator app=dremio-coordinator
	# coordinator and executors
	ddc --mode kubernetes --namespace mynamespace --coordinator app=dremio-coordinator --executors app=dremio-executor 

To sample job profiles and collect system tables information, kv reports, and Workload Manager Information add the --dremio-pat-prompt flag:

	ddc --mode kubernetes -n mynamespace -c app=dremio-coordinator -e app=dremio-executor --dremio-pat-prompt

for docker deployments
	
	# coordinator only
	ddc --mode docker -docker-path docker --coordinator localdremioincontainer
	# coordinator and executors
	ddc --mode docker -docker-path docker --coordinator localdremioincontainer --executors localdremioincontainer-executor 

`,
	Run: func(c *cobra.Command, args []string) {

	},
}

// startTicker starts a ticker that ticks every specified duration and returns
// a function that can be called to stop the ticker.
func startTicker() (stop func()) {
	ticker := time.NewTicker(time.Second * 2)
	quit := make(chan struct{})
	consoleprint.PrintState()
	go func() {
		for {
			select {
			case <-ticker.C:
				// Action to be performed on each tick
				consoleprint.PrintState()
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(quit)
	}
}

func RemoteCollect(collectionArgs collection.Args, sshArgs ssh.Args, kubeArgs kubernetes.KubeArgs, dockerArgs docker.DockerArgs, modeUsedForCollecting usedMode) error {
	consoleprint.UpdateRuntime(
		versions.GetCLIVersion(),
		simplelog.GetLogLoc(),
		collectionArgs.DDCYamlLoc,
		"",
		0,
		0,
	)
	err := validateParameters(collectionArgs, sshArgs, modeUsedForCollecting)
	if err != nil {
		fmt.Println("COMMAND HELP TEXT:")
		fmt.Println("")
		helpErr := RootCmd.Help()
		if helpErr != nil {
			return fmt.Errorf("unable to print help %w", helpErr)
		}
		return fmt.Errorf("invalid command flag detected: %w", err)
	}
	// This is where the SSH, docker or K8s collection is determined. We create an instance of the interface based on this
	// which then determines whether the commands are routed to the SSH or K8s commands
	cs, err := helpers.NewHCCopyStrategy(collectionArgs.DDCfs, &helpers.RealTimeService{})
	if err != nil {
		return fmt.Errorf("error when creating copy strategy: %v", err)
	}
	var clusterCollect = func([]string) {}
	var collectorStrategy collection.Collector
	switch modeUsedForCollecting {
	case SSH:
		simplelog.Info("using SSH based collection")
		collectorStrategy = ssh.NewCmdSSHActions(sshArgs)
	case K8S:
		simplelog.Info("using Kubernetes kubectl based collection")
		collectorStrategy = kubernetes.NewKubectlK8sActions(kubeArgs)
		clusterCollect = func(pods []string) {
			err = collection.ClusterK8sExecute(kubeArgs.Namespace, cs, collectionArgs.DDCfs, collectorStrategy, kubeArgs.KubectlPath)
			if err != nil {
				simplelog.Errorf("when getting Kubernetes info, the following error was returned: %v", err)
			}
			err = collection.GetClusterLogs(kubeArgs.Namespace, cs, collectionArgs.DDCfs, kubeArgs.KubectlPath, pods)
			if err != nil {
				simplelog.Errorf("when getting container logs, the following error was returned: %v", err)
			}
		}
	case D4R:
		simplelog.Info("using docker based collection")
		collectorStrategy = docker.NewDockerExecActions(dockerArgs)
	}

	// Launch the collection
	err = collection.Execute(collectorStrategy,
		cs,
		collectionArgs,
		clusterCollect,
	)
	if err != nil {
		return err
	}
	return nil
}

func ValidateYaml(ddcYaml string) error {
	emtpyOverrides := make(map[string]string)
	confData, err := conf.ParseConfig(ddcYaml, emtpyOverrides)
	if err != nil {
		return err
	}
	simplelog.Infof("parsed configuration for %v follows", ddcYaml)
	for k, v := range confData {
		if k == conf.KeyDremioPatToken && v != "" {
			simplelog.Infof("yaml key '%v':'REDACTED'", k)
		} else {
			simplelog.Infof("yaml key '%v':'%v'", k, v)
		}
	}
	return nil
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args []string) error {

	foundCmd, _, err := RootCmd.Find(args[1:])
	// default cmd if no cmd is given
	if err == nil && foundCmd.Use == RootCmd.Use && foundCmd.Flags().Parse(args[1:]) != pflag.ErrHelp {
		stop := startTicker()
		defer stop()
		if sshKeyLoc == "" {
			sshDefault, err := sshDefault()
			if err != nil {
				return fmt.Errorf("unexpected error getting ssh directory '%v'. This is a critical error and should result in a bug report", err)
			}
			sshKeyLoc = sshDefault
		}
		dremioPAT := ""
		if promptForDremioPAT {
			pat, err := masking.PromptForPAT()
			if err != nil {
				return fmt.Errorf("unable to get PAT due to: %v", err)
			}
			dremioPAT = pat
		}

		simplelog.Info(versions.GetCLIVersion())
		simplelog.Infof("cli command: %v", strings.Join(args, " "))
		if err := ValidateYaml(ddcYamlLoc); err != nil {
			return fmt.Errorf("CRITICAL ERROR: unable to parse %v: %v", ddcYamlLoc, err)
		}
		collectionArgs := collection.Args{
			CoordinatorStr: coordinatorStr,
			ExecutorsStr:   executorsStr,
			OutputLoc:      filepath.Clean(outputLoc),
			SudoUser:       sudoUser,
			DDCfs:          helpers.NewRealFileSystem(),
			DremioPAT:      dremioPAT,
			TransferDir:    transferDir,
			DDCYamlLoc:     ddcYamlLoc,
		}
		sshArgs := ssh.Args{
			SSHKeyLoc: sshKeyLoc,
			SSHUser:   sshUser,
		}
		kubeArgs := kubernetes.KubeArgs{
			Namespace:            namespace,
			CoordinatorContainer: coordinatorContainer,
			ExecutorsContainer:   executorsContainer,
			KubectlPath:          kubectlPath,
		}
		dockerArgs := docker.DockerArgs{
			DockerPath: dockerPath,
		}
		if err := RemoteCollect(collectionArgs, sshArgs, kubeArgs, dockerArgs, ParseString(modeUsedForCollecting)); err != nil {
			consoleprint.UpdateResult(err.Error())
		} else {
			consoleprint.UpdateResult(fmt.Sprintf("complete at %v", time.Now().Format(time.RFC1123)))
		}
		// we put the error in result so just return nil
		consoleprint.PrintState()
		return nil
	}
	if err := RootCmd.Execute(); err != nil {
		return err
	}
	return nil
}

type unableToGetHomeDir struct {
	Err error
}

func (u unableToGetHomeDir) Error() string {
	return fmt.Sprintf("unable to get home dir '%v'", u.Err)
}

// sshDefault returns the default .ssh key typically used on most deployments

func sshDefault() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", unableToGetHomeDir{}
	}
	return filepath.Join(home, ".ssh", "id_rsa"), nil
}

func init() {
	// command line flags

	RootCmd.Flags().StringVar(&coordinatorContainer, "coordinator-container", "dremio-master-coordinator,dremio-coordinator", "for use with -k8s flag: sets the container name to use to retrieve logs in the coordinators")
	RootCmd.Flags().StringVar(&executorsContainer, "executors-container", "dremio-executor", "for use with -k8s flag: sets the container name to use to retrieve logs in the executors")
	RootCmd.Flags().StringVarP(&coordinatorStr, "coordinator", "c", "", "coordinator to connect to for collection. With ssh set a list of ip addresses separated by commas. In K8s use a label that matches to the pod(s).")
	RootCmd.Flags().StringVarP(&executorsStr, "executors", "e", "", "either a common separated list or a ip range of executors nodes to connect to. With ssh set a list of ip addresses separated by commas. In K8s use a label that matches to the pod(s).")
	RootCmd.Flags().StringVarP(&sshKeyLoc, "ssh-key", "s", "", "location of ssh key to use to login")
	RootCmd.Flags().StringVarP(&sshUser, "ssh-user", "u", "", "user to use during ssh operations to login")
	RootCmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "namespace to use for kubernetes pods")
	RootCmd.Flags().StringVarP(&kubectlPath, "kubectl-path", "p", "kubectl", "where to find kubectl")
	RootCmd.Flags().StringVarP(&dockerPath, "docker-path", "d", "docker", "where to find docker")
	RootCmd.Flags().StringVarP(&modeUsedForCollecting, "mode", "m", "ssh", "use kubernetes, ssh or docker for kubernetes instead of hosts pass in labels to the --coordinator and --executors flags")
	RootCmd.Flags().BoolVarP(&promptForDremioPAT, "dremio-pat-prompt", "t", false, "Prompt for Dremio Personal Access Token (PAT)")
	RootCmd.Flags().StringVarP(&sudoUser, "sudo-user", "b", "", "if any diagnostics commands need a sudo user (i.e. for jcmd)")
	RootCmd.Flags().StringVar(&transferDir, "transfer-dir", "/tmp/ddc", "directory to use for communication between the local-collect command and this one")
	RootCmd.Flags().StringVar(&outputLoc, "output-file", "diag.tgz", "name of tgz file to save the diagnostic collection to")

	execLoc, err := os.Executable()
	if err != nil {
		fmt.Printf("unable to find ddc, critical error %v", err)
		os.Exit(1)
	}
	execLocDir := filepath.Dir(execLoc)
	RootCmd.Flags().StringVar(&ddcYamlLoc, "ddc-yaml", filepath.Join(execLocDir, "ddc.yaml"), "location of ddc.yaml that will be transferred to remote nodes for collection configuration")

	//init
	RootCmd.AddCommand(local.LocalCollectCmd)
	RootCmd.AddCommand(version.VersionCmd)
	RootCmd.AddCommand(awselogs.AWSELogsCmd)
}

func validateParameters(args collection.Args, sshArgs ssh.Args, modeUsedForCollecting usedMode) error {

	if(modeUsedForCollecting == undefined )	{
		return errors.New("the mode is not correctly set, use \"k\" or \"kubernetes\", \"d\" or \"docker\", \"s\" or \"ssh\"")
	}

	if args.CoordinatorStr == "" {
		switch modeUsedForCollecting {
		case SSH:
			return errors.New("the coordinator string was empty you must pass a single host or a comma separated lists of hosts to --coordinator or -c arguments. Example: -e 192.168.64.12,192.168.65.10")
		case K8S:
			return errors.New("the coordinator string was empty you must pass a label that will match your coordinators --coordinator or -c arguments. Example: -c \"mylabel=coordinator\"")
		case D4R:
			return errors.New("the coordinator string was empty you must pass a label that will match your coordinators container name --coordinator or -c arguments. Example: -c \"coordinator\"")
		}

	}
	if args.ExecutorsStr == "" {
		switch modeUsedForCollecting {
		case SSH:
			return errors.New("the executor string was empty you must pass a single host or a comma separated lists of hosts to --executor or -e arguments. Example: -e 192.168.64.12,192.168.65.10")
		case K8S:
			return errors.New("the executor string was empty you must pass a label that will match your executors --executor or -e arguments. Example: -e \"mylabel=executor\"")
		case D4R:
			return errors.New("the executor string was empty you must pass a label that will match your executors container --executor or -e arguments. Example: -e \"executor\"")

		}
	}

	if modeUsedForCollecting == SSH {
		if sshArgs.SSHKeyLoc == "" {
			return errors.New("the ssh private key location was empty, pass --ssh-key or -s with the key to get past this error. Example --ssh-key ~/.ssh/id_rsa")
		}
		if sshArgs.SSHUser == "" {
			return errors.New("the ssh user was empty, pass --ssh-user or -u with the user name you want to use to get past this error. Example --ssh-user ubuntu")
		}
	}
	return nil
}
