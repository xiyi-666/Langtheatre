package queue

import "context"

type TheaterTask struct {
	TheaterID string `json:"theaterId"`
	UserID    string `json:"userId"`
}

type Publisher interface {
	PublishTheaterTask(ctx context.Context, task TheaterTask) error
}

type NopPublisher struct{}

func (NopPublisher) PublishTheaterTask(_ context.Context, _ TheaterTask) error {
	return nil
}
