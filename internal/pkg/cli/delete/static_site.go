package delete

import (
	"fmt"
)

type bucketResourceGetter interface {
	BucketName(app, env, svc string) (string, error)
}

type bucketEmptier interface {
	EmptyBucket(string) error
}

// StaticSiteDeleter
type StaticSiteDeleter struct {
	BucketResourceGetter bucketResourceGetter
	BucketEmptier        bucketEmptier
}

func (s *StaticSiteDeleter) CleanResources(app, env, wkld string) error {
	bucket, err := s.BucketResourceGetter.BucketName(app, env, wkld)
	if err != nil {
		return err // TODO remove ? or check error
		return nil // allow deletion to go forward
	}

	if bucket == "" {
		// svc init'd but not deployed?
		return nil
	}

	if err := s.BucketEmptier.EmptyBucket(bucket); err != nil {
		return fmt.Errorf("empty bucket: %w", err)
	}
	return nil
}
