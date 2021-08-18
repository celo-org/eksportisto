package queue

import (
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
	"github.com/celo-org/celo-blockchain/log"
)

type QueueWriter struct {
	pubSubClient *pubsub.Client
	topic        *pubsub.Topic
}

func NewQueueWriter(projectID string, topicID string) (*QueueWriter, error) {
	ctx := context.Background()
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("pubsub.NewClient: %v", err)
	}

	return &QueueWriter{
		pubSubClient: client,
		topic:        client.Topic(topicID),
	}, nil

}

func (queue *QueueWriter) Close() error {
	return queue.pubSubClient.Close()
}

func (queue *QueueWriter) Write(message []byte) (n int, err error) {
	ctx := context.Background()
	result := queue.topic.Publish(ctx, &pubsub.Message{
		// The message also contains the new line which causes some problems upstream, so we remove that.
		Data: message[:len(message)-1],
	})
	// Block until the result is returned and a server-generated
	// ID is returned for the published message, but we can ignore
	_, err = result.Get(ctx)
	if err != nil {
		log.Error("pubsub.topic.publish Error", "err", err)
		return 0, fmt.Errorf("pubsub.topic.publish: %v", err)
	}
	return len(message), nil
}
