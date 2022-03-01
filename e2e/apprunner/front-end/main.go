// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"github.com/julienschmidt/httprouter"
	_ "github.com/lib/pq" // https://www.calhoun.io/why-we-import-sql-drivers-with-the-blank-identifier/
)

const (
	// Port is the default port number for postgres.
	Port = 5432

	postgresDriver = "postgres"
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

type Secret struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	DBName   string `json:"dbname"`
	Port     int    `json:"port"`
}

// DBGet calls an aurora DB and returns a timestamp from the database.
func DBGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	sess, err := session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_DEFAULT_REGION")),
	})
	if err != nil {
		log.Printf("🚨 initial new aws session: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ssm := secretsmanager.New(sess)
	out, err := ssm.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: aws.String(os.Getenv("FRONTENDCLUSTER_SECRET_ARN")),
	})
	if err != nil {
		log.Printf("🚨 retrieve aurora secret: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	secretValue := aws.StringValue(out.SecretString)
	if secretValue == "" {
		log.Print("🚨 empty aurora secret value\n")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	secret := Secret{}
	if err := json.Unmarshal([]byte(secretValue), &secret); err != nil {
		log.Printf("🚨 unmarshal rds secret: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	source := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s",
		secret.Host, secret.Port, secret.Username, secret.Password, secret.DBName)
	db, err := sql.Open(postgresDriver, source)
	if err != nil {
		log.Printf("🚨 sql open: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = db.Ping() // Force open a connection to the database.
	if err != nil {
		log.Printf("🚨 ping: err=%s\n", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	close := func() error {
		err := db.Close()
		if err != nil {
			return fmt.Errorf("close db: %w", err)
		}
		return nil
	}
	defer close()
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(fmt.Sprint(time.Now())))
}

func main() {
	router := httprouter.New()
	router.GET("/", SimpleGet)
	router.GET("/service-discovery-test", ServiceDiscoveryGet)
	router.GET("/db", DBGet)

	log.Fatal(http.ListenAndServe(":80", router))
}
