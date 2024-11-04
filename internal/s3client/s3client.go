// internal/s3client/s3client.go

package s3client

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// S3ClientInterface defines methods for S3 interactions
type S3ClientInterface interface {
	GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error)
	PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error)
}

// S3Client is an implementation of S3ClientInterface for AWS S3
type S3Client struct {
	s3Svc *s3.S3
}

// NewS3Client initializes a new S3 client
func NewS3Client(endpoint, region string) *S3Client {
	sess := session.Must(session.NewSession(&aws.Config{
		Endpoint: aws.String(endpoint),
		Region:   aws.String(region),
	}))
	return &S3Client{
		s3Svc: s3.New(sess),
	}
}

// GetObject retrieves an object from the specified S3 bucket
func (c *S3Client) GetObject(input *s3.GetObjectInput) (*s3.GetObjectOutput, error) {
	return c.s3Svc.GetObject(input)
}

// PutObject uploads an object to the specified S3 bucket
func (c *S3Client) PutObject(input *s3.PutObjectInput) (*s3.PutObjectOutput, error) {
	return c.s3Svc.PutObject(input)
}
