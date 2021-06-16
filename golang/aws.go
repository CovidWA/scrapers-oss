package csg

//utility functions for aws

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"sync"
)

const S3ScraperOutputBucket = "covidwa-scrapers-html"

//check if any credentials are available i nthe environment
func HasAWSCredentials() bool {
	awsConfig, err := LoadAWSConfig()
	return err == nil && awsConfig.Credentials != nil && len(awsConfig.Region) > 0
}

var awsConfigMutex *sync.Mutex = &sync.Mutex{}
var loadedAWSConfig *aws.Config

func LoadAWSConfig() (*aws.Config, error) {
	awsConfigMutex.Lock()
	defer awsConfigMutex.Unlock()

	if loadedAWSConfig == nil {
		load, err := awsconfig.LoadDefaultConfig(context.TODO())
		if err != nil {
			loadedAWSConfig = nil
			return nil, err
		}

		loadedAWSConfig = &load
	}

	return loadedAWSConfig, nil
}

func GetAWSEncryptedParameter(name string) (string, error) {
	return GetAWSParameter(name, true)
}

//get value from parameter store (aws systems manager)
func GetAWSParameter(name string, encrypted bool) (string, error) {
	cfg, err := LoadAWSConfig()
	if err != nil {
		return "", err
	}

	// Create an Amazon S3 service client
	client := ssm.NewFromConfig(*cfg)

	output, err := client.GetParameter(context.TODO(), &ssm.GetParameterInput{
		Name:           &name,
		WithDecryption: encrypted})

	if err != nil {
		return "", err
	}

	return *output.Parameter.Value, nil
}

var s3mutex *sync.Mutex = &sync.Mutex{}
var s3client *s3.Client //singleton
func PutS3Object(bucketName string, key string, body []byte) (string, error) {
	s3mutex.Lock()
	defer s3mutex.Unlock()

	if s3client == nil {
		cfg, err := LoadAWSConfig()
		if err != nil {
			return "", err
		}

		// Create an Amazon S3 service client
		s3client = s3.NewFromConfig(*cfg)
	}

	_, err := s3client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: &bucketName,
		Key:    &key,
		Body:   bytes.NewReader(body)})

	if err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://%s.s3-%s.amazonaws.com/%s", bucketName, loadedAWSConfig.Region, key)

	return url, nil
}
