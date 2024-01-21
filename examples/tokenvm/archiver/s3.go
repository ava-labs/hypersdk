package archiver

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	s3config "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

var _ Archiver = (*S3Archiver)(nil)

type S3ArchiverConfig struct {
	TokenID    string `json:"tokenID"`
	SecretKey  string `json:"secretKey"`
	Region     string `json:"region"`
	BucketName string `json:"bucketName"`
}

type S3Archiver struct {
	client     *s3.Client
	bucketName string
}

func CreateS3Archiver(ctx context.Context, configBytes []byte) (*S3Archiver, error) {
	var conf S3ArchiverConfig
	err := json.Unmarshal(configBytes, &conf)
	if err != nil {
		return nil, err
	}

	fmt.Println("initialized with conf: ", conf)

	sdkConfig, err := s3config.LoadDefaultConfig(context.TODO(),
		s3config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(conf.TokenID, conf.SecretKey, "")),
		s3config.WithRegion(conf.Region))

	if err != nil {
		return nil, err
	}

	client := s3.NewFromConfig(sdkConfig)

	return &S3Archiver{client: client, bucketName: conf.BucketName}, nil
}

func (s3a *S3Archiver) Put(k []byte, v []byte) error {
	objectKey := base64.StdEncoding.EncodeToString(k)
	objectReader := bytes.NewReader(v)

	fmt.Printf("put to bucket: %s, k=%s: v=%s with size %d \n", s3a.bucketName, objectKey, string(v), len(v))
	_, err := s3a.client.PutObject(context.TODO(), &s3.PutObjectInput{Bucket: &s3a.bucketName, Key: &objectKey, Body: objectReader})
	if err != nil {
		return err
	}

	return nil
}

func (s3a *S3Archiver) Exists(k []byte) (bool, error) {
	objectKey := string(k)

	_, err := s3a.client.HeadObject(context.TODO(), &s3.HeadObjectInput{Bucket: &s3a.bucketName, Key: &objectKey})
	if err != nil {
		var responseError *awshttp.ResponseError
		if errors.As(err, &responseError) && responseError.ResponseError.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func (s3a *S3Archiver) Get(k []byte) ([]byte, error) {
	objectKey := string(k)

	output, err := s3a.client.GetObject(context.TODO(), &s3.GetObjectInput{Bucket: &s3a.bucketName, Key: &objectKey})
	if err != nil {
		return nil, err
	}

	defer output.Body.Close()
	content, err := io.ReadAll(output.Body)
	if err != nil {
		return nil, err
	}

	return content, nil
}
