// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// SimpleGet writes the name of the service to the response.
func SimpleGet(w http.ResponseWriter, req *http.Request) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(os.Getenv("COPILOT_APPLICATION_NAME") + "-" + os.Getenv("COPILOT_ENVIRONMENT_NAME") + "-" + os.Getenv("COPILOT_SERVICE_NAME")))
}

// ProxyRequest makes a GET request to the query param "url"
// from the request and writes the response from the url to the response.
func ProxyRequest(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	log.Printf("Proxying request to %q", url)

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	handleErr := func(format string, a ...any) {
		msg := fmt.Sprintf(format, a...)
		log.Println(msg)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(msg))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		handleErr("build request: %s", err)
		return
	}

	log.Println("Built request")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		handleErr("do request: %s", err)
		return
	}
	defer resp.Body.Close()
	log.Println("Did request")

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		handleErr("read response: %s", err)
		return
	}

	log.Printf("Response:\n%s", string(body))
	w.WriteHeader(http.StatusOK)
	w.Write(body)
}

func main() {
	http.Handle("/", http.HandlerFunc(SimpleGet))
	http.Handle("/proxy", http.HandlerFunc(ProxyRequest))

	err := http.ListenAndServe(":80", nil)
	if !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("listen and serve: %s", err)
	}
}
