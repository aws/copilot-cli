// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/julienschmidt/httprouter"
)

// SimpleGet just returns true no matter what.
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("front-end oraoraora")) // NOTE: The response body has "oraoraora" appended
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

func main() {
	router := httprouter.New()
	router.GET("/", SimpleGet)
	router.GET("/service-discovery-test", ServiceDiscoveryGet)
	router.GET("/job-checker/", GetJobCheck)
	router.GET("/oraoraora-setter/", SetJobCheck) // NOTE:  "oraoraora-setter" replaces "job-setter

	log.Println("Listening on port 80...")
	log.Fatal(http.ListenAndServe(":80", router))
}
