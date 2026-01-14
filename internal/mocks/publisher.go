package mocks

import (
	"context"

	"github.com/stretchr/testify/mock"
)

type PublisherMock struct {
	mock.Mock
}

func (m *PublisherMock) Publish(ctx context.Context, routingKey string, event any) error {
	args := m.Called(ctx, routingKey, event)
	return args.Error(0)
}

func (m *PublisherMock) Close() error {
	args := m.Called()
	return args.Error(0)
}
