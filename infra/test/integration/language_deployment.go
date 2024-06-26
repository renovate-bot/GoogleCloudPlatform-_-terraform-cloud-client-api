// Copyright 2024 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package test

// language_deployment.go - common functionality for testing a deployment
// specific invocations defined in /infra/examples/*_deployment/
// which map to tests in /infra/test/integration/*_deployment/*_deployment_test.go

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/gcloud"
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/tft"
	"github.com/GoogleCloudPlatform/cloud-foundation-toolkit/infra/blueprint-test/pkg/utils"
	"github.com/gruntwork-io/terratest/modules/shell"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func AssertLanguageDeployment(t *testing.T) {
	region := regionFromEnv()

	cft := tft.NewTFBlueprintTest(t, tft.WithVars(map[string]interface{}{
		"region": region,
	}))

	cft.DefineVerify(func(assert *assert.Assertions) {
		cft.DefaultVerify(assert)

		projectID := cft.GetTFSetupStringOutput("project_id")
		serviceURL := cft.GetStringOutput("service_url")
		jobName := cft.GetStringOutput("job_name")

		// Save common arguments for gcloud calls.
		gcloudOps := gcloud.WithCommonArgs([]string{"--project", projectID, "--region", region, "--format", "json"})

		assert.Truef(strings.HasSuffix(serviceURL, ".run.app"), "unexpected service URL %q", serviceURL)

		// On initial deployment, website is serving, but has no data.
		assertResponseContains(t, assert, serviceURL, "/", "No data available.")

		// Run job
		gcloud.Run(t, fmt.Sprintf("run jobs execute  %s --wait", jobName), gcloudOps)

		// After job run, expect website to be serving valid data
		assertResponseContains(t, assert, serviceURL, "2018 Squirrel Census")
		// Check some known values to confirm data processing
		assertResponseContains(t, assert, serviceURL+"/?age=Adult&fur=Black&location=Ground+Plane", "count = 66", "points = [5,4,19,36,19]")
		assertResponseContains(t, assert, serviceURL+"/?age=Juvenile&fur=Gray&location=Above+Ground", "count = 95", "points = [13,56,22,17,13]")

		// Ensure processed files appear as they should in Cloud Storage
		// Retrieve processed bucket from service envvar
		process_job := gcloud.Run(t, fmt.Sprintf("run jobs describe %s", jobName), gcloudOps)
		processed_bucket := process_job.Get("spec.template.spec.template.spec.containers.0.env.#(name==\"PROCESSED_DATA_BUCKET\").value")

		// Use bucket name to assert objects state
		object_list := gcloud.Run(t, fmt.Sprintf("storage objects list --exhaustive gs://%s/**", processed_bucket)).Array()
		assert.Equal(len(object_list), 12, "expect twelve objects in bucket")

		// Confirm contents of a bucket
		// Uses terratest-shell directly due to space in file name
		sample_file_name := "Cinnamon/Juvenile/Ground Plane/data.json"
		args := []string{"storage", "cat", fmt.Sprintf("gs://%s/%s", processed_bucket, sample_file_name)}
		gcloudCmd := shell.Command{
			Command: "gcloud",
			Args:    append(args),
		}
		result, _ := shell.RunCommandAndGetStdOutE(t, gcloudCmd)
		sample_object := gjson.Parse(result)

		// Check all keys value  in sample object
		validationTests := map[string]struct {
			value string
		}{
			"_counter": {value: "38"},
			"Chasing":  {value: "3"},
			"Climbing": {value: "4"},
			"Eating":   {value: "16"},
			"Foraging": {value: "14"},
			"Running":  {value: "13"},
		}
		for facet, tc := range validationTests {
			assert.Equal(sample_object.Get(facet).String(), tc.value, fmt.Sprintf("field %s must match expected value", facet))
		}
	})

	cft.DefineTeardown(func(assert *assert.Assertions) {
		cft.DefaultTeardown(assert)
	})
	cft.Test()
}

///////////////////////////////////////////////////////////////////////////////////////

// Helper for calling URLs
func assertResponseContains(t *testing.T, assert *assert.Assertions, url string, text ...string) {
	t.Helper()
	var code int
	var responseBody string
	var err error

	fn := func() (bool, error) {
		t.Logf("HTTP Request - GET %s", url)
		code, responseBody, err = httpGetRequest(url)
		retry := err != nil || code < 200 || code > 299
		switch {
		case retry && err == nil:
			t.Logf("Failed HTTP Request: Status Code %d", code)
		case retry && err != nil:
			t.Logf("Failed HTTP Request: %v", err)
		default:
			// In Verbose mode with success, the asserts below are a "silent pass" during test output.
			// Facilitates real-time evaluation during long test process.
			t.Log("Successful HTTP Request")
		}
		return retry, nil
	}
	utils.Poll(t, fn, 6, 10*time.Second)

	// Assert expectations of the last checked response.
	assert.Nil(err)
	assert.GreaterOrEqual(code, 200)
	assert.LessOrEqual(code, 299)

	for _, fragment := range text {
		assert.Containsf(responseBody, fragment, "couldn't find %q in response body", fragment)
	}
}

// HTTP GET helper
func httpGetRequest(url string) (statusCode int, body string, err error) {
	res, err := http.Get(url)
	if err != nil {
		return 0, "", err
	}
	defer res.Body.Close()

	buffer, err := io.ReadAll(res.Body)
	return res.StatusCode, string(buffer), err
}

// Region gatherer
func regionFromEnv() string {
	if r := os.Getenv("GOOGLE_CLOUD_REGION"); r != "" {
		return r
	}
	return "us-central1"
}
