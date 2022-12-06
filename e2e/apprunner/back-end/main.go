// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"log"
	"net/http"
)

// HealthCheck returns a 200.
func HealthCheck(w http.ResponseWriter, req *http.Request) {
	log.Println("🚑 healthcheck ok!")
	w.WriteHeader(http.StatusOK)
}

// HelloWorld writes a hello world message.
func HelloWorld(w http.ResponseWriter, req *http.Request) {
	log.Printf("Received request to %s", req.URL.Path)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("hello-world"))
}

func main() {
	http.Handle("/", http.HandlerFunc(HealthCheck))
	http.Handle("/hello-world", http.HandlerFunc(HelloWorld))

	err := http.ListenAndServe(":80", nil)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %s", err)
	}
}
