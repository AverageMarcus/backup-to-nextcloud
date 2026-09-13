package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	CONFIG_PATH_VAR        = "NEXTCLOUD_CONFIG_PATH"
	NEXTCLOUD_USER_VAR     = "NEXTCLOUD_USER"
	NEXTCLOUD_PASSWORD_VAR = "NEXTCLOUD_PASSWORD"
)

var (
	config Config
	logger *slog.Logger
)

func init() {
	ctx := context.Background()

	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: true,
	})
	logger = slog.New(jsonHandler)
	slog.SetDefault(logger)

	configPath, exists := os.LookupEnv(CONFIG_PATH_VAR)
	if !exists {
		configPath = "./config.yaml"
	}
	logger.InfoContext(ctx,
		"Using config path",
		slog.String("configPath", configPath),
	)

	file, err := os.ReadFile(configPath)
	if err != nil {
		logger.ErrorContext(ctx, "Error reading config file", slog.Any("error", err))
		os.Exit(1)
		return
	}

	err = yaml.Unmarshal(file, &config)
	if err != nil {
		logger.ErrorContext(ctx, "Error parsing config file", slog.Any("error", err))
		os.Exit(1)
		return
	}

	switch {
	case os.Getenv(NEXTCLOUD_USER_VAR) == "":
		logger.ErrorContext(ctx, "NEXTCLOUD_USER environment variable is not set")
		os.Exit(1)
		return
	case os.Getenv(NEXTCLOUD_PASSWORD_VAR) == "":
		logger.ErrorContext(ctx, "NEXTCLOUD_PASSWORD environment variable is not set")
		os.Exit(1)
		return
	}
}

func main() {
	ctx := context.Background()

	if err := config.Validate(); err != nil {
		logger.ErrorContext(ctx, "Invalid config", slog.Any("error", err))
		os.Exit(1)
		return
	}

	client := NewClient(config.NextcloudURL, os.Getenv(NEXTCLOUD_USER_VAR), os.Getenv(NEXTCLOUD_PASSWORD_VAR))

	for _, job := range config.Jobs {
		err := processJob(ctx, client, job)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to process job",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("destinationDirectory", job.DestinationDirectory),
				slog.Any("error", err),
			)
			continue
		} else {
			logger.DebugContext(ctx, "Finished processing job",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("destinationDirectory", job.DestinationDirectory),
			)
		}
	}
}

func processJob(ctx context.Context, client *Client, job Job) error {
	// 1. Check that we actually have files to upload...
	files, err := os.ReadDir(job.SourceDirectory)
	if err != nil {
		logger.ErrorContext(ctx, "Failed to read source directory",
			slog.String("sourceDirectory", job.SourceDirectory),
			slog.Any("error", err),
		)
		return err
	}

	if len(files) == 0 {
		logger.InfoContext(ctx, "No files found in source directory, skipping job",
			slog.String("sourceDirectory", job.SourceDirectory),
		)
		return nil
	}

	// 2. Ensure the destination exists in Nextcloud
	if err := client.CreateDestDirectory(ctx, job.DestinationDirectory); err != nil {
		logger.ErrorContext(ctx, "Failed to create destination directory in Nextcloud",
			slog.String("destinationDirectory", job.DestinationDirectory),
			slog.Any("error", err),
		)
		return err
	}

	// 3. Upload files
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		fileName := filepath.Base(file.Name())
		filePath := fmt.Sprintf("%s/%s", job.SourceDirectory, fileName)

		file, err := os.Open(filePath)
		if err != nil {
			logger.ErrorContext(ctx, "Error opening file",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("fileName", fileName),
				slog.String("filePath", filePath),
				slog.Any("error", err),
			)
			return err
		}
		defer file.Close()

		logger.DebugContext(ctx, "Uploading file",
			slog.String("sourceDirectory", job.SourceDirectory),
			slog.String("destinationDirectory", job.DestinationDirectory),
			slog.String("fileName", fileName),
			slog.String("filePath", filePath),
		)
		if err := client.UploadFile(ctx, job.DestinationDirectory, file); err != nil {
			logger.ErrorContext(ctx, "Failed to upload file",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("destinationDirectory", job.DestinationDirectory),
				slog.String("fileName", fileName),
				slog.String("filePath", filePath),
				slog.Any("error", err),
			)
			return err
		}
	}

	// 4. Remove any files older than the max age specified
	if job.MaxAge != nil {
		duration, _ := time.ParseDuration(*job.MaxAge)
		threshold := time.Now().Add(-duration)
		logger.InfoContext(ctx, "Removing files older than max age",
			slog.String("destinationDirectory", job.DestinationDirectory),
			slog.String("maxAge", *job.MaxAge),
			slog.Time("dateThreshold", threshold),
		)

		contents, err := client.ListDirectoryContents(ctx, job.DestinationDirectory)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to list directory contents",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("destinationDirectory", job.DestinationDirectory),
				slog.Any("error", err),
			)
			return err
		}

		for _, file := range contents {
			if file.LastModified != nil {
				if file.LastModified.Before(threshold) {
					logger.InfoContext(ctx, "Removing old file",
						slog.String("destinationDirectory", job.DestinationDirectory),
						slog.String("fileName", file.FileName),
						slog.Time("lastModified", *file.LastModified),
					)
					if err := client.DeleteFile(ctx, job.DestinationDirectory, file.FileName); err != nil {
						logger.ErrorContext(ctx, "Failed to delete file",
							slog.String("destinationDirectory", job.DestinationDirectory),
							slog.String("fileName", file.FileName),
							slog.Any("error", err),
						)
						return err
					}
				}
			}
		}
	}

	// 5. Ensure only newest X files are kept
	if job.MaxItems != nil {
		logger.InfoContext(ctx, "Removing excess files",
			slog.String("destinationDirectory", job.DestinationDirectory),
			slog.Int("maxItem", *job.MaxItems),
		)

		contents, err := client.ListDirectoryContents(ctx, job.DestinationDirectory)
		if err != nil {
			logger.ErrorContext(ctx, "Failed to list directory contents",
				slog.String("sourceDirectory", job.SourceDirectory),
				slog.String("destinationDirectory", job.DestinationDirectory),
				slog.Any("error", err),
			)
			return err
		}

		sort.SliceStable(contents, func(i, j int) bool {
			if contents[i].LastModified == nil && contents[j].LastModified == nil {
				return false
			}
			if contents[i].LastModified == nil {
				return false // nil goes to the end
			}
			if contents[j].LastModified == nil {
				return true // nil goes to the end
			}
			return contents[i].LastModified.After(*contents[j].LastModified) // flipped: After instead of Before
		})

		if len(contents) > *job.MaxItems {
			for _, file := range contents[*job.MaxItems:] {
				logger.InfoContext(ctx, "Removing old files until max files met",
					slog.String("destinationDirectory", job.DestinationDirectory),
					slog.String("fileName", file.FileName),
					slog.Time("lastModified", *file.LastModified),
					slog.String("maxItems", *job.MaxAge),
				)
				if err := client.DeleteFile(ctx, job.DestinationDirectory, file.FileName); err != nil {
					logger.ErrorContext(ctx, "Failed to delete file",
						slog.String("destinationDirectory", job.DestinationDirectory),
						slog.String("fileName", file.FileName),
						slog.String("maxItems", *job.MaxAge),
						slog.Any("error", err),
					)
					return err
				}
			}
		}
	}

	return nil
}
