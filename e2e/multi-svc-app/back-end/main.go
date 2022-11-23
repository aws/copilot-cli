// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"

	"github.com/julienschmidt/httprouter"
)

// HealthCheck just returns true if the service is up.
func HealthCheck(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("🚑 healthcheck ok!")
	w.WriteHeader(http.StatusOK)
}

// SimpleGet just returns true no matter what.
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("back-end"))
}

// Get just returns true no matter what.
func Get(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get on service endpoint Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("back-end-service"))
}

func main() {
	router := httprouter.New()
	router.GET("/back-end/", SimpleGet)
	router.GET("/service-endpoint/", Get)

	// Health Check
	router.GET("/", HealthCheck)

	log.Fatal(http.ListenAndServe(":80", router))
}
