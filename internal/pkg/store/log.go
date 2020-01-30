package store

import (
	"time"

	"github.com/aws/amazon-ecs-cli-v2/internal/pkg/archer"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func (s *Store) GetLog(appName, startTime string) (*[]archer.LogEntry, error) {
	output, err := s.cwClient.GetLogEvents(&cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String(""),
		LogStreamName: aws.String(""),
		//NextToken:     nil,
		StartFromHead: aws.Bool(true),
		//StartTime:     nil,
	})
	if err != nil {
		return nil, err
	}

	var events []archer.LogEntry
	for _, e := range output.Events {
		sec := *e.Timestamp / 1000
		localTime := time.Unix(sec, 0).Local().Format(time.RFC3339)
		events = append(events,	archer.LogEntry{Timestamp: localTime, Message: *e.Message})
	}
	return &events, nil
}
