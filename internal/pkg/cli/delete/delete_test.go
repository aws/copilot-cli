package delete

import (
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/copilot-cli/internal/pkg/cli/delete/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

type ecrEmptierMocks struct {
	imageRemover    *mocks.MockimageRemover
	sessionProvider *mocks.MockregionalSessionProvider
}

func TestEmptyRepo(t *testing.T) {
	mockRepo := "asdf"
	tests := map[string]struct {
		setupMocks  func(m *ecrEmptierMocks)
		regions     map[string]struct{}
		expectedErr string
	}{
		"fail to get session for us-west-2": {
			setupMocks: func(m *ecrEmptierMocks) {
				m.sessionProvider.EXPECT().DefaultWithRegion("us-west-2").Return(nil, errors.New("mock error"))
			},
			regions: map[string]struct{}{
				"us-west-2": {},
			},
			expectedErr: "mock error",
		},
		"fail to clear us-west-2": {
			setupMocks: func(m *ecrEmptierMocks) {
				m.sessionProvider.EXPECT().DefaultWithRegion("us-west-2").Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
				m.imageRemover.EXPECT().ClearRepository(mockRepo).Return(errors.New("mock error"))
			},
			regions: map[string]struct{}{
				"us-west-2": {},
			},
			expectedErr: "mock error",
		},
		"success clearing us-west-2": {
			setupMocks: func(m *ecrEmptierMocks) {
				m.sessionProvider.EXPECT().DefaultWithRegion("us-west-2").Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
				m.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil)
			},
			regions: map[string]struct{}{
				"us-west-2": {},
			},
		},
		"success clearing us-west-2 and us-east-1": {
			setupMocks: func(m *ecrEmptierMocks) {
				m.sessionProvider.EXPECT().DefaultWithRegion("us-west-2").Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-west-2"),
					},
				}, nil)
				m.sessionProvider.EXPECT().DefaultWithRegion("us-east-1").Return(&session.Session{
					Config: &aws.Config{
						Region: aws.String("us-east-1"),
					},
				}, nil)
				m.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil)
				m.imageRemover.EXPECT().ClearRepository(mockRepo).Return(nil)
			},
			regions: map[string]struct{}{
				"us-west-2": {},
				"us-east-1": {},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			mocks := &ecrEmptierMocks{
				imageRemover:    mocks.NewMockimageRemover(ctrl),
				sessionProvider: mocks.NewMockregionalSessionProvider(ctrl),
			}
			tc.setupMocks(mocks)

			ecrEmptier := &ECREmptier{
				SessionProvider: mocks.sessionProvider,
				newImageRemover: func(sess *session.Session) imageRemover {
					return mocks.imageRemover
				},
			}

			err := ecrEmptier.EmptyRepo(mockRepo, tc.regions)
			if tc.expectedErr != "" {
				require.EqualError(t, err, tc.expectedErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
