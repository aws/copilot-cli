// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"log"
	"net/http"
)

var message = "hello world"

// HealthCheck just returns true if the service is up.
func HealthCheck(w http.ResponseWriter, req *http.Request) {
	log.Println("🚑 healthcheck ok!")
	w.WriteHeader(http.StatusOK)
}

// ServiceDiscoveryGet just returns true no matter what.
func ServiceDiscoveryGet(w http.ResponseWriter, req *http.Request) {
	log.Printf("Get on ServiceDiscovery endpoint Succeeded with message %s\n", message)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(message))
}

func main() {
	http.Handle("/", http.HandlerFunc(HealthCheck))
	http.Handle("/service-discovery", http.HandlerFunc(ServiceDiscoveryGet))

	err := http.ListenAndServe(":80", nil)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %s", err)
	}
}
