package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

type DB_S3 struct {
	DB_core

	bucket string
	path   string
	client *s3.Client
}

func NewDB_S3(u *url.URL, is_writable bool) (DB, *DB_core, error) {
	if cfg, err := config.LoadDefaultConfig(context.TODO()); err != nil {
		return nil, nil, err
	} else {
		db := &DB_S3{
			DB_core: newDB_Core(is_writable),
			bucket:  u.Host,
			path:    strings.TrimPrefix(u.Path, "/"),
			client:  s3.NewFromConfig(cfg),
		}
		return db, &db.DB_core, nil
	}
}

func (d *DB_S3) Get(key string, ignore_missing bool) ([]byte, error) {
	key = filepath.Join(d.path, key)
	slog.Debug(`db read`, "file", fmt.Sprintf(`s3://%s/%s`, d.bucket, key))

	res, err := d.client.GetObject(context.TODO(), &s3.GetObjectInput{
		Bucket:       aws.String(d.bucket),
		Key:          aws.String(key),
		ChecksumMode: types.ChecksumModeEnabled,
	})

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey":
			if ignore_missing {
				return nil, nil
			} else {
				return nil, fmt.Errorf(`key "%s" not found on S3`, key)
			}
		case "Unauthorized":
			return nil, fmt.Errorf(`unauthorized access to S3`)
		}
	}

	if err != nil {
		return nil, fmt.Errorf(`error while comunicating with S3. %v`, apiErr)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(res.Body)
	res.Body.Close()

	return buf.Bytes(), nil
}

func (d *DB_S3) Put(key string, val []byte, ignore_existing bool) error {
	key = filepath.Join(d.path, key)
	slog.Debug(`db write`, "file", fmt.Sprintf(`s3://%s/%s`, d.bucket, key))

	var condition *string
	if !ignore_existing {
		condition = aws.String("*")
	}

	_, err := d.client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:            aws.String(d.bucket),
		Key:               aws.String(key),
		Body:              bytes.NewReader(val),
		IfNoneMatch:       condition,
		ChecksumAlgorithm: types.ChecksumAlgorithmCrc32,
	})

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "PreconditionFailed":
			return fmt.Errorf(`key "%s" already exists on S3`, key)
		case "Unauthorized":
			return fmt.Errorf(`unauthorized access to S3`)
		}
	}

	if err != nil {
		return fmt.Errorf(`error while comunicating with S3. %v`, err)
	}

	return nil
}

func (d *DB_S3) AtomicPut(key string, val []byte) error {
	return d.Put(key, val, true)
}

func (d *DB_S3) Rm(key string) error {
	key = filepath.Join(d.path, key)
	slog.Debug(`db delete`, "file", fmt.Sprintf(`s3://%s/%s`, d.bucket, key))

	_, err := d.client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(d.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (d *DB_S3) Mkdir() error {
	return nil
}
