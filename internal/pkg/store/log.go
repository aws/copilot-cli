package store

import (
	"fmt"
	"regexp"
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func (s *Store) GetLog(logID string, startTime int64) (*[]archer.LogEntry, int64, error) {
	var entries []archer.LogEntry
	var nextToken *string

	endTime := time.Now().UnixNano() / 1e6
	for {
		newEntries, nextToken, err := s.getLog(logID, nextToken, startTime, endTime)
		if err != nil {
			return nil, 0, err
		}
		entries = append(entries, newEntries...)

		if nextToken == nil {
			break
		}
	}
	return &entries, endTime, nil
}

func (s *Store) getLog(logID string, nextToken *string, startTime, endTime int64) ([]archer.LogEntry, *string, error) {
	output, err := s.cwClient.FilterLogEvents(&cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(fmt.Sprintf("/ecs/%s", logID)),
		NextToken:    nextToken,
		StartTime:    aws.Int64(startTime),
		EndTime:      aws.Int64(endTime),
	})
	if err != nil {
		return nil, nil, err
	}

	var events []archer.LogEntry
	for _, e := range output.Events {
		re := regexp.MustCompile(`ecs/(.*)/(.*)`)
		streamName := re.FindStringSubmatch(*e.LogStreamName)[2]

		event := archer.LogEntry{
			Timestamp:  *e.Timestamp,
			StreamName: streamName,
			Message:    *e.Message,
		}
		events = append(events, event)
	}
	return events, output.NextToken, nil
}
