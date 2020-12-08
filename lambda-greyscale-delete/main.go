package main

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/sirupsen/logrus"
)

func handler(ctx context.Context, event events.S3Event) {
	logger := logrus.WithFields(logrus.Fields{"action": "converter"})

	sess := session.Must(session.NewSession())
	s3svc := s3.New(sess)
	snsSvc := sns.New(sess)

	for _, e := range event.Records {
		handleDeletedObject(e, s3svc, snsSvc, logger)
	}

	logger.Infof("lambda function finished, processed '%d' events", len(event.Records))
}

func handleDeletedObject(object events.S3EventRecord, s3svc *s3.S3, snsSvc *sns.SNS, logger *logrus.Entry) {
	imageSourceBucket := object.S3.Bucket.Name
	imageSourceKey := object.S3.Object.Key

	imageDestinationBucket := fmt.Sprintf("%s-convert", imageSourceBucket)
	imageDestinationKey := fmt.Sprintf("converted-%s", imageSourceKey)

	// remove item from converted bucket
	_, err := s3svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(imageDestinationBucket),
		Key: aws.String(imageDestinationKey),
	})
	if err != nil {
		handleError(errors.Wrapf(err, "error deleting object from bucket"), snsSvc)
		return
	}
	logger.Infof("successfully removed %s from bucket %s", imageDestinationKey, imageDestinationBucket)
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