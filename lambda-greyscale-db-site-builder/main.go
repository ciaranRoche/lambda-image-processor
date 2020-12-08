package main

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pkg/errors"
	"os"
	"text/template"

	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"

)

type GreyImage struct {
	SourceBucket string `json:"sourceBucket"`
	SourceKey string `json:"sourceKey"`
	SourceURL string `json:"sourceURL"`
	ConvertBucket string `json:"convertBucket"`
	ConvertKey string `json:"convertKey"`
	ConvertURL string `json:"convertURL"`
	ImageType string `json:"imageType"`
}

const (
	websiteBucket = "greyscale-website"
	websiteKey    = "index.html"
	tableName = "Image"
	snsTopic = "arn:aws:sns:eu-west-1:442832839294:websiteUpdated"
)

func handler(ctx context.Context, e events.DynamoDBEvent) {
	logger := logrus.WithFields(logrus.Fields{"action": "builder"})
	logger.Info("lambda greyscale site builder function called")

	// create session
	sess := session.Must(session.NewSession())
	s3uploader := s3manager.NewUploader(sess)
	dynoSvc := dynamodb.New(sess)
	snsSvc := sns.New(sess)

	for _, record := range e.Records {
		logger.Infof("processing %s for event ID %s", record.EventName, record.EventID)

		filter := expression.Name("convertURL").AttributeExists()

		expr , err := expression.NewBuilder().WithFilter(filter).Build()
		if err != nil {
			handleError(errors.Wrapf(err, "error building dynamodb expression"), snsSvc)
			return
		}

		result, err := dynoSvc.Scan(&dynamodb.ScanInput{
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			TableName:                 aws.String(tableName),
		})
		if err != nil {
			handleError(errors.Wrapf(err, "error getting items from dynamodb"), snsSvc)
			return
		}

		var images []string
		for _, item := range result.Items {
			image := GreyImage{}

			err = dynamodbattribute.UnmarshalMap(item, &image)
			if err != nil {
				handleError(errors.Wrapf(err, "error unmarshalling image"), snsSvc)
				return
			}
			logger.Infof("found image url %s", image.ConvertURL)

			images = append(images, image.ConvertURL)
		}

		logger.Infof("found %d images", len(images))

		buf := bytes.NewBufferString("")

		tpl, err := template.ParseFiles("index.gohtml")
		if err != nil {
			handleError(errors.Wrapf(err, "error parsing template"), snsSvc)
			return
		}

		err = tpl.Execute(buf, images)
		if err != nil {
			handleError(errors.Wrapf(err, "unable to parse html template"), snsSvc)
			return
		}

		r := bytes.NewReader(buf.Bytes())

		_, err = s3uploader.Upload(&s3manager.UploadInput{
			Bucket: aws.String(websiteBucket),
			Key: aws.String(websiteKey),
			Body: r,
			ContentType: aws.String("text/html"),
		})
		if err != nil {
			handleError(errors.Wrapf(err, "error putting html in bucket"), snsSvc)
			return
		}

		// public sns message
		snsMessage := fmt.Sprintf("website updated with '%d' images", len(images))
		logger.Infof("sending message : %s", snsMessage)
		_, err = snsSvc.Publish(&sns.PublishInput{
			Message: aws.String(string(snsMessage)),
			TopicArn: aws.String(snsTopic),
		})
		if err != nil {
			handleError(errors.Wrapf(err, "error publishing sns"), snsSvc)
			return
		}

		logger.Infof("finished updating website for event %s", record.EventID)
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