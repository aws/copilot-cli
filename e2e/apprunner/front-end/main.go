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

// SimpleGet just returns true no matter what
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(os.Getenv("COPILOT_APPLICATION_NAME") + "-" + os.Getenv("COPILOT_ENVIRONMENT_NAME") + "-" + os.Getenv("COPILOT_SERVICE_NAME")))
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

func main() {
	router := httprouter.New()
	router.GET("/", SimpleGet)
	router.GET("/service-discovery-test", ServiceDiscoveryGet)

	log.Fatal(http.ListenAndServe(":80", router))
}
