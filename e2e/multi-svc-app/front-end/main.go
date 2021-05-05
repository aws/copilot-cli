// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	"github.com/julienschmidt/httprouter"
)

// Get the env var "MAGIC_WORDS" for testing if the build arg was overridden.
var magicWords string = os.Getenv("MAGIC_WORDS")
var volumeName string = "efsTestVolume"

// SimpleGet just returns true no matter what
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("front-end"))
}

// ServiceDiscoveryGet calls the back-end service, via service-discovery.
// This call should succeed and return the value from the backend service.
// This test assumes the backend app is called "back-end". The 'service-discovery' endpoint
// of the back-end service is unreachable from the LB, so the only way to get it is
// through service discovery. The response should be `back-end-service-discovery`
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	endpoint := fmt.Sprintf("http://back-end.%s/service-discovery/", os.Getenv("COPILOT_SERVICE_DISCOVERY_ENDPOINT"))
	resp, err := http.Get(endpoint)
	if err != nil {
		log.Printf("🚨 could call service discovery endpoint: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Println("Get on ServiceDiscovery endpoint Succeeded")
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

// GetMagicWords returns the environment variable passed in by the arg override
func GetMagicWords(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	log.Println(magicWords)
	w.Write([]byte(magicWords))
}

// GetJobCheck returns the value of the environment variable TEST_JOB_CHECK_VAR.
func GetJobCheck(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get /job-checker/ succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(os.Getenv("TEST_JOB_CHECK_VAR")))
}

// SetJobCheck updates the environment variable TEST_JOB_CHECK_VAR in the container to "yes"
func SetJobCheck(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get /job-setter/ succeeded")
	err := os.Setenv("TEST_JOB_CHECK_VAR", "yes")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// PutEFSCheck writes a file to the EFS folder in the container.
func PutEFSCheck(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	efsVar := os.Getenv("COPILOT_MOUNT_POINTS")
	copilotMountPoints := make(map[string]string)
	if err := json.Unmarshal([]byte(efsVar), &copilotMountPoints); err != nil {
		log.Println("Unmarshal COPILOT_MOUNT_POINTS env var FAILED")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fileName := fmt.Sprintf("%s/testfile", copilotMountPoints[volumeName])
	fileObj, err := os.Create(fileName)
	if err != nil {
		log.Printf("Create test file %s in EFS volume FAILED\n", fileName)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// Resize file to 10M
	if err := fileObj.Truncate(1e7); err != nil {
		log.Printf("Resize test file %s in EFS volume FAILED\n", fileName)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	fileObj.Close()

	// Shred file to ensure metered size increases and it's not read as sparse.
	shredCmd := exec.Command("shred", "-n", "1", fileName)
	if err := shredCmd.Run(); err != nil {
		log.Println("Shred test file in EFS volume FAILED")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fi, err := os.Stat(fileName)
	if err != nil {
		log.Printf("Get info for file %s\n", fileName)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Printf("File size: %d\n", fi.Size())
	log.Println("Get /efs-putter succeeded")
	w.WriteHeader(http.StatusOK)
}

func main() {
	router := httprouter.New()
	router.GET("/", SimpleGet)
	router.GET("/service-discovery-test", ServiceDiscoveryGet)
	router.GET("/magicwords/", GetMagicWords)
	router.GET("/job-checker/", GetJobCheck)
	router.GET("/job-setter/", SetJobCheck)
	router.GET("/efs-putter", PutEFSCheck)

	log.Println("Listening on port 80...")
	log.Fatal(http.ListenAndServe(":80", router))
}
