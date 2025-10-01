package pulsar

import (
	"fmt"
	"log"

	"github.com/pulsar-local-lab/perf-test/internal/config"
	pulsaradmin "github.com/streamnative/pulsar-admin-go"
	"github.com/streamnative/pulsar-admin-go/pkg/utils"
)

// EnsureTopic ensures that the specified topic exists with the correct partition configuration.
// If the topic doesn't exist, it creates it with the specified number of partitions.
// If the topic exists, it verifies the partition count matches the configuration.
func EnsureTopic(cfg *config.Config) error {
	// Create admin client
	adminCfg := &pulsaradmin.Config{
		WebServiceURL: cfg.Pulsar.AdminURL,
	}

	admin, err := pulsaradmin.NewClient(adminCfg)
	if err != nil {
		return fmt.Errorf("failed to create admin client: %w", err)
	}

	// Parse topic name
	topicName, err := utils.GetTopicName(cfg.Pulsar.Topic)
	if err != nil {
		return fmt.Errorf("invalid topic name %s: %w", cfg.Pulsar.Topic, err)
	}

	// Check if topic exists
	exists, err := topicExists(admin, topicName)
	if err != nil {
		return fmt.Errorf("failed to check topic existence: %w", err)
	}

	if exists {
		// Topic exists - verify partition count
		if cfg.Pulsar.TopicPartitions > 0 {
			// Check if it's a partitioned topic
			metadata, err := admin.Topics().GetMetadata(*topicName)
			if err != nil {
				// If we can't get metadata, assume it's compatible
				log.Printf("Warning: could not verify topic partition count: %v", err)
				return nil
			}

			if metadata.Partitions != cfg.Pulsar.TopicPartitions {
				return fmt.Errorf("topic exists with %d partitions, but config specifies %d partitions. Delete the topic or change the partition count",
					metadata.Partitions, cfg.Pulsar.TopicPartitions)
			}
			log.Printf("Topic %s exists with %d partitions", cfg.Pulsar.Topic, cfg.Pulsar.TopicPartitions)
		} else {
			log.Printf("Topic %s exists (non-partitioned)", cfg.Pulsar.Topic)
		}
		return nil
	}

	// Topic doesn't exist - create it
	if cfg.Pulsar.TopicPartitions > 0 {
		// Create partitioned topic
		log.Printf("Creating partitioned topic %s with %d partitions", cfg.Pulsar.Topic, cfg.Pulsar.TopicPartitions)
		err = admin.Topics().Create(*topicName, cfg.Pulsar.TopicPartitions)
		if err != nil {
			return fmt.Errorf("failed to create partitioned topic: %w", err)
		}
		log.Printf("Successfully created partitioned topic %s", cfg.Pulsar.Topic)
	} else {
		// Create non-partitioned topic (will be auto-created by producer/consumer)
		log.Printf("Topic %s will be auto-created as non-partitioned", cfg.Pulsar.Topic)
	}

	return nil
}

// topicExists checks if a topic exists (either partitioned or non-partitioned)
func topicExists(admin pulsaradmin.Client, topicName *utils.TopicName) (bool, error) {
	// Try to get topic metadata - if it succeeds, topic exists
	_, err := admin.Topics().GetMetadata(*topicName)
	if err != nil {
		// Check if error indicates topic doesn't exist
		// The admin API returns an error for non-existent topics
		// We'll assume any error means the topic doesn't exist
		return false, nil
	}
	return true, nil
}
