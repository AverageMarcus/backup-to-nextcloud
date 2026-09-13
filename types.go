package main

import (
	"fmt"
	"time"
)

type Config struct {
	NextcloudURL string `yaml:"nextcloudURL"`
	Jobs         []Job  `yaml:"jobs"`
}

type Job struct {
	SourceDirectory      string  `yaml:"sourceDirectory"`
	DestinationDirectory string  `yaml:"destinationDirectory"`
	MaxItems             *int    `yaml:"maxItems"`
	MaxAge               *string `yaml:"maxAge"`
}

func (c Config) Validate() error {
	if c.NextcloudURL == "" {
		return fmt.Errorf("nextcloudURL is required")
	}
	for _, job := range c.Jobs {
		if job.SourceDirectory == "" {
			return fmt.Errorf("sourceDirectory is required for job")
		}
		if job.DestinationDirectory == "" {
			return fmt.Errorf("destinationDirectory is required for job")
		}
		if job.MaxItems != nil && *job.MaxItems <= 0 {
			return fmt.Errorf("maxItems must be greater than 0 for job")
		}
		if job.MaxAge != nil {
			if _, err := time.ParseDuration(*job.MaxAge); err != nil {
				return fmt.Errorf("maxAge must be a valid duration for job - %v", err)
			}
		}
	}
	return nil
}
