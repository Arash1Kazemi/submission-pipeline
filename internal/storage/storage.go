// Package storage wraps the MinIO client shared by every pipeline tool.
//
// It moves bytes and nothing else: it knows no bucket names, no key formats,
// and nothing about submissions. Callers supply both, so that naming decisions
// live with the code that understands them.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// ErrNotFound reports a missing bucket or object. Handlers should treat it as
// a terminal rejection rather than a retryable failure — an object that is not
// there now will not appear on the next attempt.
var ErrNotFound = errors.New("object not found")

// TooLargeError reports an object that exceeds the configured size limit. Like
// ErrNotFound this is terminal: the file will be exactly as large next time.
type TooLargeError struct {
	Bucket string
	Key    string
	Size   int64
	Limit  int64
}

func (e *TooLargeError) Error() string {
	return fmt.Sprintf("%s/%s is %d bytes, limit is %d", e.Bucket, e.Key, e.Size, e.Limit)
}

// Client is a MinIO connection plus the size ceiling every download is held to.
type Client struct {
	mc       *minio.Client
	maxBytes int64
}

// New connects to MinIO and verifies the credentials work. maxBytes caps every
// download; pass 0 to disable the check.
func New(ctx context.Context, endpoint, accessKey, secretKey string, useSSL bool, maxBytes int64) (*Client, error) {
	mc, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("creating minio client: %w", err)
	}

	// minio.New does not talk to the server, so without this a bad endpoint or
	// wrong key only surfaces on the first real job.
	if _, err := mc.ListBuckets(ctx); err != nil {
		return nil, fmt.Errorf("connecting to minio at %s: %w", endpoint, err)
	}

	return &Client{mc: mc, maxBytes: maxBytes}, nil
}

// Stat returns an object's metadata without transferring its contents.
func (c *Client) Stat(ctx context.Context, bucket, key string) (minio.ObjectInfo, error) {
	info, err := c.mc.StatObject(ctx, bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return minio.ObjectInfo{}, translate(bucket, key, err)
	}
	return info, nil
}

// Exists reports whether an object is present.
func (c *Client) Exists(ctx context.Context, bucket, key string) (bool, error) {
	_, err := c.Stat(ctx, bucket, key)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// Download writes an object to dstPath and returns the number of bytes written.
//
// The size is checked before any transfer starts, so an oversized object costs
// one metadata request rather than a full download.
func (c *Client) Download(ctx context.Context, bucket, key, dstPath string) (int64, error) {
	info, err := c.Stat(ctx, bucket, key)
	if err != nil {
		return 0, err
	}
	if c.maxBytes > 0 && info.Size > c.maxBytes {
		return 0, &TooLargeError{Bucket: bucket, Key: key, Size: info.Size, Limit: c.maxBytes}
	}

	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return 0, translate(bucket, key, err)
	}
	defer obj.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return 0, fmt.Errorf("creating %s: %w", dstPath, err)
	}

	n, copyErr := io.Copy(dst, c.capped(obj))
	// Buffered writes can fail at close — a disk that fills during the copy
	// reports the error here, not from io.Copy. Ignoring it leaves a truncated
	// file that looks complete.
	closeErr := dst.Close()

	switch {
	case copyErr != nil:
		os.Remove(dstPath)
		return 0, fmt.Errorf("downloading %s/%s: %w", bucket, key, copyErr)
	case closeErr != nil:
		os.Remove(dstPath)
		return 0, fmt.Errorf("writing %s: %w", dstPath, closeErr)
	case c.maxBytes > 0 && n > c.maxBytes:
		// Reached only if the reported size was wrong; the cap is enforced on
		// the actual bytes regardless of what the metadata claimed.
		os.Remove(dstPath)
		return 0, &TooLargeError{Bucket: bucket, Key: key, Size: n, Limit: c.maxBytes}
	}
	return n, nil
}

// Open returns the object as a stream, for consumers that read sequentially and
// never need the whole file at once. The caller must Close it.
func (c *Client) Open(ctx context.Context, bucket, key string) (io.ReadCloser, error) {
	info, err := c.Stat(ctx, bucket, key)
	if err != nil {
		return nil, err
	}
	if c.maxBytes > 0 && info.Size > c.maxBytes {
		return nil, &TooLargeError{Bucket: bucket, Key: key, Size: info.Size, Limit: c.maxBytes}
	}

	obj, err := c.mc.GetObject(ctx, bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, translate(bucket, key, err)
	}
	return &cappedReader{rc: obj, remaining: c.remaining(), bucket: bucket, key: key, limit: c.maxBytes}, nil
}

// Upload stores a local file under key and returns the number of bytes sent.
// contentType may be empty, in which case MinIO stores it as binary data.
func (c *Client) Upload(ctx context.Context, bucket, key, srcPath, contentType string) (int64, error) {
	info, err := c.mc.FPutObject(ctx, bucket, key, srcPath, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return 0, fmt.Errorf("uploading %s to %s/%s: %w", srcPath, bucket, key, err)
	}
	return info.Size, nil
}

// UploadStream stores whatever r yields under key. Pass size = -1 when the
// length is unknown; MinIO then buffers internally to pick a part size, which
// costs memory, so prefer passing a real size when you have one.
func (c *Client) UploadStream(ctx context.Context, bucket, key string, r io.Reader, size int64, contentType string) (int64, error) {
	info, err := c.mc.PutObject(ctx, bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return 0, fmt.Errorf("uploading to %s/%s: %w", bucket, key, err)
	}
	return info.Size, nil
}

func (c *Client) remaining() int64 {
	if c.maxBytes <= 0 {
		return -1
	}
	return c.maxBytes
}

// capped limits a reader to maxBytes+1, so that io.Copy stops just past the
// limit and the caller can tell "exactly at the limit" from "over it".
func (c *Client) capped(r io.Reader) io.Reader {
	if c.maxBytes <= 0 {
		return r
	}
	return io.LimitReader(r, c.maxBytes+1)
}

// cappedReader enforces the size limit on a stream whose length was not known
// in advance, turning an overrun into a typed error rather than silent truncation.
type cappedReader struct {
	rc        io.ReadCloser
	remaining int64 // -1 disables the check
	bucket    string
	key       string
	limit     int64
}

func (c *cappedReader) Read(p []byte) (int, error) {
	if c.remaining == 0 {
		return 0, &TooLargeError{Bucket: c.bucket, Key: c.key, Size: c.limit, Limit: c.limit}
	}
	if c.remaining > 0 && int64(len(p)) > c.remaining {
		p = p[:c.remaining]
	}
	n, err := c.rc.Read(p)
	if c.remaining > 0 {
		c.remaining -= int64(n)
	}
	return n, err
}

func (c *cappedReader) Close() error { return c.rc.Close() }

// translate converts MinIO's error responses into this package's sentinel
// errors, so callers never import minio just to check for a missing object.
func translate(bucket, key string, err error) error {
	resp := minio.ToErrorResponse(err)
	switch resp.Code {
	case "NoSuchKey", "NoSuchBucket", "NotFound":
		return fmt.Errorf("%s/%s: %w", bucket, key, ErrNotFound)
	}
	return fmt.Errorf("%s/%s: %w", bucket, key, err)
}
