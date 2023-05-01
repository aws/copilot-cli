package delete

import (
	"errors"
	"fmt"

	"github.com/aws/copilot-cli/internal/pkg/s3"
)

type bucketResourceGetter interface {
	BucketName(app, env, svc string) (string, error)
}

type bucketEmptier interface {
	EmptyBucket(string) error
}

type StaticSiteDeleter struct {
	BucketResourceGetter bucketResourceGetter
	BucketEmptier        bucketEmptier
}

func (s *StaticSiteDeleter) CleanResources(app, env, wkld string) error {
	bucket, err := s.BucketResourceGetter.BucketName(app, env, wkld)
	if err != nil {
		var notFound *s3.ErrNotFound
		if errors.As(err, &notFound) {
			// bucket doesn't exist, no need to clean up
			return nil
		}
		return fmt.Errorf("get bucket name: %w", err)
	}

	if err := s.BucketEmptier.EmptyBucket(bucket); err != nil {
		return fmt.Errorf("empty bucket: %w", err)
	}
	return nil
}
