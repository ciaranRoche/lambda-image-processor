package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	image2 "image"
	"image/jpeg"
	"io"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/ciaranRoche/greyscale/pkg/imageprocessing"
	"github.com/sirupsen/logrus"
)

const (
	region = "eu-west-1"
)

type GreyImage struct {
	SourceBucket  string `json:"sourceBucket"`
	SourceKey     string `json:"sourceKey"`
	SourceURL     string `json:"sourceURL"`
	ConvertBucket string `json:"convertBucket"`
	ConvertKey    string `json:"convertKey"`
	ConvertURL    string `json:"convertURL"`
	ImageType     string `json:"imageType"`
}

func handler(ctx context.Context, event events.S3Event) {
	logger := logrus.WithFields(logrus.Fields{"action": "converter"})

	sess := session.Must(session.NewSession())
	s3svc := s3.New(sess)
	s3uploader := s3manager.NewUploader(sess)
	snsSvc := sns.New(sess)

	for _, e := range event.Records {
		if err := handleNewObject(e, s3svc, s3uploader, snsSvc, logger); err != nil {
			handleError(err, snsSvc)
			continue
		}
	}

	logger.Infof("lambda function finished, processed '%d' events", len(event.Records))
}

func handleNewObject(object events.S3EventRecord, s3svc *s3.S3, s3uploader *s3manager.Uploader, snsSvc *sns.SNS, logger *logrus.Entry) error {
	imageSourceBucket := object.S3.Bucket.Name
	imageSourceKey := object.S3.Object.Key

	imageDestinationBucket := fmt.Sprintf("%s-convert", imageSourceBucket)
	imageDestinationKey := fmt.Sprintf("converted-%s", imageSourceKey)

	// get uploaded image
	logger.Infof("getting image '%s' from bucket '%s'", imageSourceKey, imageSourceBucket)
	img, err := s3svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(imageSourceBucket),
		Key:    aws.String(imageSourceKey),
	})
	if err != nil {
		logger.Errorf("error getting image from bucket : %v", err)
		return err
	}

	imgType := aws.StringValue(img.ContentType)

	// convert image to buffer
	imageBufferCopy := &bytes.Buffer{}
	_, err = io.Copy(imageBufferCopy, img.Body)
	if err != nil {
		logger.Errorf("error creating buffer copy : %v", err)
		return err
	}

	// decode buffer to image type
	logger.Infof("decoding buffer of size %d", len(imageBufferCopy.Bytes()))
	decodedImage, _, err := image2.Decode(imageBufferCopy)
	if err != nil {
		logger.Errorf("error decoding buffer : %v", err)
		return err
	}

	// create image processing pipeline
	logger.Infof("imageprocessor starting for image %s ", imageSourceKey)
	processorPipeline := imageprocessing.NewProcessorPipeline()
	greyScaleAction := imageprocessing.NewActionGreyScale()
	processorPipeline.AddAction(greyScaleAction)
	processedImage, err := processorPipeline.Transform(decodedImage)
	if err != nil {
		logger.Errorf("error processing image %v", err)
		return err
	}
	logger.Infof("imageprocessor ended for image %s ", imageSourceKey)

	// encode converted image
	logger.Infof("encoding image %s", imageSourceKey)
	var b bytes.Buffer
	imageWriter := bufio.NewWriter(&b)
	err = jpeg.Encode(imageWriter, processedImage, &jpeg.Options{Quality: 100})
	if err != nil {
		logger.Errorf("error encoding image: %v ", err)
		return err
	}

	// upload converted image to converted image bucket
	logger.Infof("uploading image %s to bucket %s", imageDestinationKey, imageDestinationBucket)
	_, err = s3uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(imageDestinationBucket),
		Key:         aws.String(imageDestinationKey),
		Body:        &b,
		ContentType: aws.String("image/png"),
	})
	if err != nil {
		logger.Errorf("error putting image in bucket : %v", err)
		return err
	}

	// create sns topic for successful image conversion
	snsMessage, err := json.Marshal(GreyImage{
		SourceBucket:  imageSourceBucket,
		SourceKey:     imageSourceKey,
		SourceURL:     buildImageUrl(imageSourceBucket, region, imageSourceKey),
		ConvertBucket: imageDestinationBucket,
		ConvertKey:    imageDestinationKey,
		ConvertURL:    buildImageUrl(imageDestinationBucket, region, imageDestinationKey),
		ImageType:     imgType,
	})
	if err != nil {
		logger.Errorf("error, invalid json : %v", err)
		return err
	}

	// public sns message
	logger.Infof("sending message : %s", string(snsMessage))
	_, err = snsSvc.Publish(&sns.PublishInput{
		Message:  aws.String(string(snsMessage)),
		TopicArn: aws.String(os.Getenv("TOPICSNS")),
	})
	if err != nil {
		logger.Errorf("error publishing sns : %v", err)
		return err
	}
	return nil
}

func handleError(err error, snsSvc *sns.SNS) {
	log := logrus.WithFields(logrus.Fields{"action": "error"})
	log.Error(err)

	// publish error message to sns topic
	_, err = snsSvc.Publish(&sns.PublishInput{
		Message:  aws.String(fmt.Sprintf("error : %v", err)),
		TopicArn: aws.String(os.Getenv("ERRORSNS")),
	})
	if err != nil {
		// fail gracefully
		log.Errorf("error publishing sns : %v", err)
	}
}

func buildImageUrl(bucket, region, key string) string {
	return fmt.Sprintf("https://%s.s3-%s.amazonaws.com/%s", bucket, region, key)
}

func main() {
	lambda.Start(handler)
}
