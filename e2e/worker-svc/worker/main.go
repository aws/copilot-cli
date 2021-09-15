// Copyright Amazon.com, Inc. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package worker

import (
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/aws/copilot-cli/internal/pkg/aws/sessions"
)

var (
	queueURIVarName      = "COPILOT_QUEUE_URI"
	topicQueueURIVarName = "COPILOT_TOPIC_QUEUE_URIS"
)

func main() {
	p := sessions.NewProvider()
	sess, err := p.Default()
	if err != nil {
		log.Fatal(err)
	}
	client := sqs.New(sess)
	queueURI := os.Getenv(queueURIVarName)

	for {
		go func() {
			resp, err := client.ReceiveMessage(
				&sqs.ReceiveMessageInput{
					MaxNumberOfMessages: 10,
					AttributeNames: []*string{aws.String("All")},
					QueueUrl: aws.String(queueURI),
					WaitTimeSeconds: aws.Int64(1),
				},
			)

			for _, msg := range resp.Messages {
				switch aws.StringValue(msg.Body) {
				case "good message":
					client.DeleteMessage(&sqs.DeleteMessageInput{
						QueueUrl: aws.String(queueURI),
						ReceiptHandle: msg.ReceiptHandle,
					})
				case "bad message":
					continue
				}
			}
		}()

		go func() {
			client.ReceiveMessage(

			)
		}
	}
	// Pull messages for main queue.
	

	// Launch messages for `events-topic-specific` topic.
	go func() {
		time.Sleep(60 * time.Second)
		for a := 0; a < 100; a++ {

			time.Sleep(5 * time.Second)
		}
	}()

}
