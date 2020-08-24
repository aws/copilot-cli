// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/julienschmidt/httprouter"
)

var (
	sess = session.Must(session.NewSession())
	svc  = dynamodb.New(sess)

	tableName = os.Getenv("HELLO_TABLE_NAME")
)

// HealthCheck validates that the environment variables are available before we say the service is healthy.
func HealthCheck(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	_, hasTableEnv := os.LookupEnv("HELLO_TABLE_NAME")
	if !hasTableEnv {
		log.Println("🚨HELLO_TABLE_NAME environment variable is missing!")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, hasSecretEnv := os.LookupEnv("PASSWORD_SECRET")
	if !hasSecretEnv {
		log.Println("🚨PASSWORD_SECRET environment variable is missing!")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("🚑 healthcheck ok!")
	w.WriteHeader(http.StatusOK)
}

func Hello(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	input := &dynamodb.PutItemInput{
		Item: map[string]*dynamodb.AttributeValue{
			"Name": {
				S: aws.String(ps.ByName("name")),
			},
		},
		TableName: aws.String(tableName),
	}
	if _, err := svc.PutItem(input); err != nil {
		log.Printf("put item %v: %v\n", input, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func ListHello(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	input := &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	}
	out, err := svc.Scan(input)
	if err != nil {
		log.Printf("😭 scan items for table %s: %v", tableName, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	var names []string
	for _, item := range out.Items {
		names = append(names, aws.StringValue(item["Name"].S))
	}
	data, err := json.Marshal(struct {
		Names []string
	}{
		Names: names,
	})
	if err != nil {
		log.Printf("marshal json: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func main() {
	router := httprouter.New()
	router.POST("/hello/:name", Hello)
	router.GET("/hello", ListHello)

	// Health Check
	router.GET("/", HealthCheck)

	log.Fatal(http.ListenAndServe(":80", router))
}
