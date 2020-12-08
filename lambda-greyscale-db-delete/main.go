package main

import (
	"context"
	"fmt"
	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"os"
)


const tableName = "Image"


type GreyImage struct {
	ImageConverter string `json:"imageConverter"`
	SourceBucket string `json:"sourceBucket"`
	SourceKey string `json:"sourceKey"`
	SourceURL string `json:"sourceURL"`
	ConvertBucket string `json:"convertBucket"`
	ConvertKey string `json:"convertKey"`
	ConvertURL string `json:"convertURL"`
	ImageType string `json:"imageType"`
}

func handler(ctx context.Context, event events.S3Event) {
	logger := logrus.WithFields(logrus.Fields{"action": "converter"})
	logger.Info("lambda greyscale db function called")

	// create session
	sess := session.Must(session.NewSession())
	dynoSvc := dynamodb.New(sess)
	snsSvc := sns.New(sess)


	// handle all records from sqs event
	for _, e := range event.Records {
		logger.Infof("received event %s for event source %s", e.EventName, e.EventSource)

		imageKey := e.S3.Object.Key

		filter := expression.Name("convertKey").Equal(expression.Value(imageKey))

		expr , err := expression.NewBuilder().WithFilter(filter).Build()
		if err != nil {
			handleError(errors.Wrapf(err, "error building dynamodb expression"), snsSvc)
			continue
		}

		result, err := dynoSvc.Scan(&dynamodb.ScanInput{
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			FilterExpression:          expr.Filter(),
			TableName:                 aws.String(tableName),
		})
		if err != nil {
			handleError(errors.Wrapf(err, "error getting items from dynamodb"), snsSvc)
			continue
		}

		for _, item := range result.Items {
			image := GreyImage{}

			err = dynamodbattribute.UnmarshalMap(item, &image)
			if err != nil {
				handleError(errors.Wrapf(err, "error unmarshalling image"), snsSvc)
				continue
			}
			logger.Infof("found image url %s", image.ConvertURL)

			imgConverterValue := image.ImageConverter
			_, err := dynoSvc.DeleteItem(&dynamodb.DeleteItemInput{
				Key: map[string]*dynamodb.AttributeValue{
					"imageConverter": {
						S: aws.String(imgConverterValue),
					},
				},
				TableName: aws.String("Image"),
			})
			if err != nil {
				handleError(errors.Wrapf(err, "error deleting item from DB"), snsSvc)
				continue
			}

		}
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
