// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package publisher

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"

	"github.com/julienschmidt/httprouter"
)

// SimpleGet just returns true no matter what
func SimpleGet(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Get Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("publisher"))
}

func SimplePost(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

}

func healthCheckHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	log.Println("Health Check Succeeded")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("publisher"))
}

func main() {
	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		log.Fatal(err)
	}
	client := sns.New(sess)
	topicARNVars := os.Getenv("COPILOT_SNS_TOPIC_ARNS")
	snsTopics := make(map[string]string)
	err = json.Unmarshal([]byte(topicARNVars), &snsTopics)
	if err != nil {
		log.Fatal(err)
	}

	// Launch messages for `events` topic.
	go func() {
		time.Sleep(60 * time.Second)
		for a := 0; a < 100; a++ {
			client.Publish(
				&sns.PublishInput{
					TopicArn: aws.String(snsTopics["events"]),
					Message:  aws.String("good message"),
				},
			)
			time.Sleep(5 * time.Second)
		}
	}()

	// Launch poison pill messages for `events` topic.
	go func() {
		time.Sleep(60 * time.Second)
		for a := 0; a < 10; a++ {
			resp, err := client.Publish(
				&sns.PublishInput{
					TopicArn: aws.String(snsTopics["events"]),
					Message:  aws.String("bad message"),
				},
			)
			if err != nil {
				log.Printf("error sending message: %s", err.Error())
			} else {
				log.Print(resp)
			}
			time.Sleep(30 * time.Second)
		}
	}()
	// Launch messages for `events-topic-specific` topic.
	go func() {
		time.Sleep(60 * time.Second)
		for a := 0; a < 100; a++ {
			client.Publish(
				&sns.PublishInput{
					TopicArn: aws.String(snsTopics["events-topic-specific"]),
					Message:  aws.String("good message"),
				},
			)
			time.Sleep(5 * time.Second)
		}
	}()

	router := httprouter.New()
	router.POST("/post", SimplePost)
	router.GET("/", healthCheckHandler)
	log.Fatal(http.ListenAndServe(":80", router))
}
