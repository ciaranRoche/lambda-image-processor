package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"io"
	"os"
)

func handler(ctx context.Context, e events.S3Event) {
	logger := logrus.WithFields(logrus.Fields{"action": "backup"})
	logger.Info("lambda greyscale site backup function called")

	// create session
	sess := session.Must(session.NewSession())
	s3svc := s3.New(sess)
	s3uploader := s3manager.NewUploader(sess)
	snsSvc := sns.New(sess)

	for _, record := range e.Records {
		websiteSourceBucket := record.S3.Bucket.Name
		websiteKey := record.S3.Object.Key
		websiteDestinationBucket := fmt.Sprintf("%s-backup", websiteSourceBucket)

		website, err := s3svc.GetObject(&s3.GetObjectInput{
			Bucket: aws.String(websiteSourceBucket),
			Key:    aws.String(websiteKey),
		})
		if err != nil {
			handleError(errors.Wrap(err, "error getting image from bucket"), snsSvc)
			return
		}

		// convert image to buffer
		websiteBufferCopy := &bytes.Buffer{}
		_, err = io.Copy(websiteBufferCopy, website.Body)
		if err != nil {
			handleError(errors.Wrap(err, "error creating buffer copy"), snsSvc)
			return
		}

		_, err = s3uploader.Upload(&s3manager.UploadInput{
			Bucket:      aws.String(websiteDestinationBucket),
			Key:         aws.String(websiteKey),
			Body:        websiteBufferCopy,
			ContentType: aws.String("image/png"),
		})
		if err != nil {
			handleError(errors.Wrap(err, "error putting image in bucket"), snsSvc)
			return
		}


		logger.Infof("finished updating website for event %s", record.EventName)
	}
}

func handleError(err error, snsSvc *sns.SNS) {
	log := logrus.WithFields(logrus.Fields{"action": "error"})
	log.Error(err)

	// publish error message to sns topic
	_, err = snsSvc.Publish(&sns.PublishInput{
		Message: aws.String(fmt.Sprintf("error : %v", err)),
		TopicArn: aws.String(os.Getenv("ERRORSNS")),
	})
	if err != nil {
		// fail gracefully
		log.Errorf("error publishing sns : %v", err)
	}
}

func main() {
	lambda.Start(handler)
}