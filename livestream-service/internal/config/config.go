package config

import (
	"fmt"
	"strings"

	"github.com/spf13/viper"
)

// Config holds all runtime configuration for the service.
type Config struct {
	Port                  string
	Env                   string
	AWSRegion             string
	AWSEndpointURL        string
	UseMemoryStore        bool
	UseMockIVS            bool
	DynamoTableStreams    string
	DynamoTableChat       string
	DynamoTableTickets    string
	SQSQueueURL           string
	IVSWebhookSecret      string
	S3BucketRecordings    string
	CloudFrontDomain      string
	CloudFrontKeyPairID   string
	CloudFrontPrivateKey  string
	LaravelInternalURL    string
	LaravelInternalSecret string
}

// Load reads configuration from environment variables and optional .env file.
func Load() (*Config, error) {
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	_ = viper.ReadInConfig()

	setDefault("PORT", "8080")
	setDefault("ENV", "development")
	setDefault("AWS_REGION", "us-east-1")
	setDefault("DYNAMODB_TABLE_STREAMS", "streams")
	setDefault("DYNAMODB_TABLE_CHAT", "chat_messages")
	setDefault("DYNAMODB_TABLE_TICKETS", "tickets")

	cfg := &Config{
		Port:                  viper.GetString("PORT"),
		Env:                   viper.GetString("ENV"),
		AWSRegion:             viper.GetString("AWS_REGION"),
		AWSEndpointURL:        viper.GetString("AWS_ENDPOINT_URL"),
		UseMemoryStore:        viper.GetBool("USE_MEMORY_STORE"),
		UseMockIVS:            viper.GetBool("USE_MOCK_IVS"),
		DynamoTableStreams:    viper.GetString("DYNAMODB_TABLE_STREAMS"),
		DynamoTableChat:       viper.GetString("DYNAMODB_TABLE_CHAT"),
		DynamoTableTickets:    viper.GetString("DYNAMODB_TABLE_TICKETS"),
		SQSQueueURL:           viper.GetString("SQS_QUEUE_URL"),
		IVSWebhookSecret:      viper.GetString("IVS_WEBHOOK_SECRET"),
		S3BucketRecordings:    viper.GetString("S3_BUCKET_RECORDINGS"),
		CloudFrontDomain:      viper.GetString("CLOUDFRONT_DOMAIN"),
		CloudFrontKeyPairID:   viper.GetString("CLOUDFRONT_KEY_PAIR_ID"),
		CloudFrontPrivateKey:  viper.GetString("CLOUDFRONT_PRIVATE_KEY_PATH"),
		LaravelInternalURL:    viper.GetString("LARAVEL_INTERNAL_URL"),
		LaravelInternalSecret: viper.GetString("LARAVEL_INTERNAL_SECRET"),
	}

	if cfg.LaravelInternalURL == "" {
		return nil, fmt.Errorf("missing LARAVEL_INTERNAL_URL")
	}

	return cfg, nil
}

func setDefault(key, value string) {
	if !viper.IsSet(key) {
		viper.Set(key, value)
	}
}
