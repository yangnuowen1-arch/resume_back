package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/yangnuowen1-arch/resume_back/internal/filemime"
)

type UploadResult struct {
	Key string
	URL string
}

type Object struct {
	Body          io.ReadCloser
	ContentType   string
	ContentLength *int64
}

type Uploader interface {
	Upload(ctx context.Context, key string, file *multipart.FileHeader, contentType string) (*UploadResult, error)
	UploadBytes(ctx context.Context, key string, data []byte, contentType string) (*UploadResult, error)
	Open(ctx context.Context, key string) (*Object, error)
	Delete(ctx context.Context, key string) error
}

// RangeOpener is implemented by storage backends that can fetch just one byte
// range of an object. Keeping it separate from Uploader preserves compatibility
// with existing upload-only fakes while allowing PDF viewers to load large
// documents progressively.
type RangeOpener interface {
	OpenRange(ctx context.Context, key string, start, end int64) (*Object, error)
}

type LocalUploader struct {
	root string
}

func NewLocalUploader(root string) *LocalUploader {
	return &LocalUploader{
		root: root,
	}
}

func (u *LocalUploader) Upload(ctx context.Context, key string, file *multipart.FileHeader, contentType string) (*UploadResult, error) {
	source, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()

	targetPath := filepath.Join(u.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, err
	}

	target, err := os.Create(targetPath)
	if err != nil {
		return nil, err
	}
	defer target.Close()

	if _, err := io.Copy(target, source); err != nil {
		return nil, err
	}

	return &UploadResult{
		Key: key,
		URL: "/" + filepath.ToSlash(targetPath),
	}, nil
}

// UploadBytes — 直接写入内存字节（供邮箱附件等非 multipart 来源使用）。
func (u *LocalUploader) UploadBytes(ctx context.Context, key string, data []byte, contentType string) (*UploadResult, error) {
	targetPath := filepath.Join(u.root, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, err
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil {
		return nil, err
	}

	return &UploadResult{
		Key: key,
		URL: "/" + filepath.ToSlash(targetPath),
	}, nil
}

// LocalUploader.Open — 从本地文件系统读取文件
func (u *LocalUploader) Open(ctx context.Context, key string) (*Object, error) {
	file, err := os.Open(filepath.Join(u.root, filepath.FromSlash(key)))
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	size := stat.Size()
	return &Object{
		Body:          file,
		ContentLength: &size,
	}, nil
}

// OpenRange opens an inclusive byte range without reading the rest of a local
// file. The caller is responsible for validating the HTTP Range header; this
// method still checks bounds to avoid returning a misleading partial stream.
func (u *LocalUploader) OpenRange(ctx context.Context, key string, start, end int64) (*Object, error) {
	if start < 0 || end < start {
		return nil, fmt.Errorf("invalid byte range %d-%d", start, end)
	}

	file, err := os.Open(filepath.Join(u.root, filepath.FromSlash(key)))
	if err != nil {
		return nil, err
	}

	stat, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	if start >= stat.Size() || end >= stat.Size() {
		_ = file.Close()
		return nil, fmt.Errorf("byte range %d-%d is outside object size %d", start, end, stat.Size())
	}
	if _, err := file.Seek(start, io.SeekStart); err != nil {
		_ = file.Close()
		return nil, err
	}

	length := end - start + 1
	return &Object{
		Body: &limitedReadCloser{
			Reader: io.LimitReader(file, length),
			Closer: file,
		},
		ContentLength: &length,
	}, nil
}

type limitedReadCloser struct {
	io.Reader
	io.Closer
}

func (u *LocalUploader) Delete(ctx context.Context, key string) error {
	return os.Remove(filepath.Join(u.root, filepath.FromSlash(key)))
}

type R2Config struct {
	Endpoint        string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
}

type R2Uploader struct {
	client        *s3.Client
	bucket        string
	publicBaseURL string
}

func NewR2Uploader(ctx context.Context, r2Config R2Config) (*R2Uploader, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			r2Config.AccessKeyID,
			r2Config.SecretAccessKey,
			"",
		)),
		awsconfig.WithRegion("auto"),
	)
	if err != nil {
		return nil, err
	}

	endpoint := normalizeEndpoint(r2Config.Endpoint)
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
	})

	return &R2Uploader{
		client:        client,
		bucket:        r2Config.Bucket,
		publicBaseURL: strings.TrimRight(r2Config.PublicBaseURL, "/"),
	}, nil
}

func (u *R2Uploader) Upload(ctx context.Context, key string, file *multipart.FileHeader, contentType string) (*UploadResult, error) {
	source, err := file.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()

	contentType = normalizeUploadContentType(key, file.Filename, contentType, file.Header.Get("Content-Type"))
	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        source,
		ContentType: aws.String(contentType),
	}

	if _, err := u.client.PutObject(ctx, input); err != nil {
		return nil, err
	}

	return &UploadResult{
		Key: key,
		URL: u.objectURL(key),
	}, nil
}

// UploadBytes — 直接写入内存字节（供邮箱附件等非 multipart 来源使用）。
func (u *R2Uploader) UploadBytes(ctx context.Context, key string, data []byte, contentType string) (*UploadResult, error) {
	contentType = normalizeUploadContentType(key, "", contentType)
	input := &s3.PutObjectInput{
		Bucket:      aws.String(u.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	}

	if _, err := u.client.PutObject(ctx, input); err != nil {
		return nil, err
	}

	return &UploadResult{
		Key: key,
		URL: u.objectURL(key),
	}, nil
}

// R2Uploader.Open — 从云存储（Cloudflare R2 / S3 兼容）读取文件
func (u *R2Uploader) Open(ctx context.Context, key string) (*Object, error) {
	output, err := u.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	contentType := ""
	if output.ContentType != nil {
		contentType = *output.ContentType
	}

	return &Object{
		Body:          output.Body,
		ContentType:   contentType,
		ContentLength: output.ContentLength,
	}, nil
}

// OpenRange asks the S3-compatible backend to return exactly the requested
// inclusive range. This prevents a PDF.js range request from downloading the
// entire object through the application server.
func (u *R2Uploader) OpenRange(ctx context.Context, key string, start, end int64) (*Object, error) {
	if start < 0 || end < start {
		return nil, fmt.Errorf("invalid byte range %d-%d", start, end)
	}

	output, err := u.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
		Range:  aws.String(fmt.Sprintf("bytes=%d-%d", start, end)),
	})
	if err != nil {
		return nil, err
	}

	contentType := ""
	if output.ContentType != nil {
		contentType = *output.ContentType
	}

	return &Object{
		Body:          output.Body,
		ContentType:   contentType,
		ContentLength: output.ContentLength,
	}, nil
}

func (u *R2Uploader) Delete(ctx context.Context, key string) error {
	_, err := u.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(u.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (u *R2Uploader) objectURL(key string) string {
	if u.publicBaseURL == "" {
		return fmt.Sprintf("r2://%s/%s", u.bucket, key)
	}

	result, err := url.JoinPath(u.publicBaseURL, key)
	if err != nil {
		return u.publicBaseURL + "/" + key
	}

	return result
}

func normalizeEndpoint(endpoint string) string {
	parsed, err := url.Parse(strings.TrimSpace(endpoint))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return strings.TrimRight(endpoint, "/")
	}

	parsed.Path = ""
	parsed.RawPath = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	return strings.TrimRight(parsed.String(), "/")
}

func normalizeUploadContentType(key, filename string, contentTypes ...string) string {
	fallbackName := filename
	if strings.TrimSpace(fallbackName) == "" {
		fallbackName = key
	}

	candidates := append([]string{}, contentTypes...)
	candidates = append(candidates, filepath.Ext(filename), filepath.Ext(key))
	return filemime.NormalizeAny(fallbackName, candidates...)
}
