package store

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/route53"
)

func (s *Store) CreateCNAME(source, target string) error {
	return s.changeRecordSets("UPSERT", source, target)
}

func (s *Store) DeleteCNAME(source, target string) error {
	return s.changeRecordSets("DELETE", source, target)
}

func (s *Store) changeRecordSets(action, source, target string) error {
	hostedZone, err := s.getHostedZone()
	if err != nil {
		return err
	}

	_, err = s.route53Full.ChangeResourceRecordSets(&route53.ChangeResourceRecordSetsInput{
		ChangeBatch: &route53.ChangeBatch{
			Changes: []*route53.Change{
				{
					Action: aws.String(action),
					ResourceRecordSet: &route53.ResourceRecordSet{
						Name: aws.String(target),
						ResourceRecords: []*route53.ResourceRecord{
							{
								Value: aws.String(source),
							},
						},
						TTL:  aws.Int64(300),
						Type: aws.String("CNAME"),
					},
				},
			},
		},
		HostedZoneId: aws.String(hostedZone),
	})
	return err
}

func (s *Store) getHostedZone() (string, error) {
	name := "dw.run."
	output, err := s.route53Full.ListHostedZonesByName(&route53.ListHostedZonesByNameInput{
		DNSName: aws.String(name),
	})
	if err != nil {
		return "", err
	}
	// should be the first result since we're passing DNSName but just in case
	re := regexp.MustCompile(`/hostedzone/(.*)`)
	for _, hostedZone := range output.HostedZones {
		if *hostedZone.Name == name {
			id := re.FindStringSubmatch(*hostedZone.Id)[1]
			return id, nil
		}
	}
	return "", errors.New(fmt.Sprintf("%s was not found in this account", name))
}
