package imageprocessing

import "image"

//go:generate moq -out imageaction_moq_test.go . ImageAction
type ImageAction interface {
	Transform(image.Image) (image.Image, error)
}