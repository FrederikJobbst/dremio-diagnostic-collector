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

// apicollect provides all the methods that collect via the API, this is a substantial part of the activities of DDC so it gets it's own package
package apicollect

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dremio/dremio-diagnostic-collector/cmd/local/conf"
	"github.com/dremio/dremio-diagnostic-collector/cmd/local/ddcio"
	"github.com/dremio/dremio-diagnostic-collector/cmd/local/restclient"
	"github.com/dremio/dremio-diagnostic-collector/pkg/simplelog"
)

func RunCollectDremioSystemTables(c *conf.CollectConf) error {
	simplelog.Info("Collecting results from Export System Tables...")
	err := ValidateAPICredentials(c)
	if err != nil {
		return err
	}
	// TODO: Row limit and sleem MS need to be configured
	rowlimit := 100000
	sleepms := 100

	for _, systable := range c.Systemtables() {
		filename := "sys." + strings.Replace(systable, "\\\"", "", -1) + ".json"
		body, err := downloadSysTable(c, systable, rowlimit, sleepms)
		if err != nil {
			return err
		}
		dat := make(map[string]interface{})
		err = json.Unmarshal(body, &dat)
		if err != nil {
			return fmt.Errorf("unable to unmarshall JSON response - %w", err)
		}
		if err == nil {
			rowcount := dat["returnedRowCount"].(float64)
			if int(rowcount) == rowlimit {
				simplelog.Warning("Returned row count for sys." + systable + " has been limited to " + strconv.Itoa(rowlimit))
			}
		}
		sb := string(body)
		systemTableFile := path.Join(c.SystemTablesOutDir(), filename)
		file, err := os.Create(path.Clean(systemTableFile))
		if err != nil {
			return fmt.Errorf("unable to create file %v due to error %v", filename, err)
		}
		defer ddcio.EnsureClose(filepath.Clean(systemTableFile), file.Close)
		_, err = fmt.Fprint(file, sb)
		if err != nil {
			return fmt.Errorf("unable to create file %s due to error %v", filename, err)
		}
		simplelog.Info("SUCCESS - Created " + filename)
	}

	return nil
}

func downloadSysTable(c *conf.CollectConf, systable string, rowlimit int, sleepms int) ([]byte, error) {
	// TODO: Consider using official api/v3, requires paging of job results
	headers := map[string]string{"Content-Type": "application/json"}
	sqlurl := c.DremioEndpoint() + "/api/v3/sql"
	joburl := c.DremioEndpoint() + "/api/v3/job/"
	jobid, err := restclient.PostQuery(sqlurl, c.DremioPATToken(), headers, systable)
	if err != nil {
		return nil, err
	}
	jobstateurl := joburl + jobid
	jobstate := "RUNNING"
	for jobstate == "RUNNING" {
		time.Sleep(time.Duration(sleepms) * time.Millisecond)
		body, err := restclient.APIRequest(jobstateurl, c.DremioPATToken(), "GET", headers)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve job state from %s due to error %v", jobstateurl, err)
		}
		dat := make(map[string]interface{})
		err = json.Unmarshal(body, &dat)
		if err != nil {
			return nil, fmt.Errorf("unable to unmarshall JSON response - %w", err)
		}
		jobstate = dat["jobState"].(string)
	}
	if jobstate == "COMPLETED" {
		jobresultsurl := c.DremioEndpoint() + "/apiv2/job/" + jobid + "/data?offset=0&limit=" + strconv.Itoa(rowlimit)
		simplelog.Info("Retrieving job results ...")
		body, err := restclient.APIRequest(jobresultsurl, c.DremioPATToken(), "GET", headers)
		if err != nil {
			return nil, fmt.Errorf("unable to retrieve job results from %s due to error %v", jobresultsurl, err)
		}
		return body, nil
	}
	return nil, fmt.Errorf("unable to retrieve job results for sys." + systable)
}