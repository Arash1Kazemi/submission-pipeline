// Package storage wraps the MinIO client shared by every pipeline tool
package storage

type Client struct {
	// TODO: wraps *minio.Client (github.com/minio/minio-go/v7)
}

func NewClient(endpoint, accessKey, secretKey string, useSSL bool) (*Client, error) {
	panic("TODO")
}

func (c *Client) Upload(bucket, objectKey, srcPath string) error {
	panic("TODO")
}

func (c *Client) Download(bucket, objectKey, datPath string) error {
	panic("TODO")
}
