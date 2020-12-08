package imageprocessing

import (
	"errors"
	"fmt"
	"image"
)

//go:generate moq -out pipeline_moq_test.go . ProcessorPipeline
type ProcessorPipeline interface {
	AddAction(ImageAction)
	Transform(image.Image) (image.Image, error)
}

var _ ProcessorPipeline = &processorPipeline{}

type processorPipeline struct {
	imageProcesses []ImageAction
}

func NewProcessorPipeline() ProcessorPipeline {
	return &processorPipeline{
		imageProcesses: []ImageAction{},
	}
}

func (p *processorPipeline) AddAction(action ImageAction) {
	if action == nil {
		return
	}
	p.imageProcesses = append(p.imageProcesses, action)
}

func (p processorPipeline) Transform(image image.Image) (image.Image, error) {
	if image == nil {
		return nil, errors.New("image should not be nil")
	}
	currentImage := image
	for _, processor := range p.imageProcesses {
		var err error
		currentImage, err = processor.Transform(currentImage)
		if err != nil {
			return nil, fmt.Errorf("failed to transform image: %w", err)
		}
	}
	return currentImage, nil
}
