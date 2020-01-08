package store

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ssm"
)

func (s *Store) CreateSecret(secretName, secretString string) (string, error) {
	output, err := s.ssmClient.PutParameter(&ssm.PutParameterInput{
		Name:        aws.String(secretName),
		Overwrite:   aws.Bool(true),
		Type:        aws.String(ssm.ParameterTypeSecureString),
		Value:       aws.String(secretString),
	})
	return string(*output.Version), err
}
