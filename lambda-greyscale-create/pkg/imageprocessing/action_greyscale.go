package imageprocessing

import (
	"fmt"
	"image"
	image2 "image"
	"image/color"
	_ "image/png"
	"sync"
)

type actionGreyScale struct{}

var _ ImageAction = actionGreyScale{}

func NewActionGreyScale() ImageAction {
	return &actionGreyScale{}
}

func (a actionGreyScale) Transform(image image.Image) (image.Image, error) {
	// convert image to 3 dimensional array
	size:= image.Bounds().Size()
	var pixels [][]color.Color
	//put pixels into two three two dimensional array
	for i:=0; i<size.X;i++{
		var y []color.Color
		for j:=0; j<size.Y;j++{
			y = append(y,image.At(i,j))
		}
		pixels = append(pixels,y)
	}

	// convert pixels to greyScale
	convertedPixels := greyScale(pixels)

	// convert RGBA back to image
	rect := image2.Rect(0,0,len(convertedPixels),len(convertedPixels[0]))
	nImg := image2.NewRGBA(rect)

	for x:=0; x<len(convertedPixels);x++{
		for y:=0; y<len(convertedPixels[0]);y++ {
			q:=convertedPixels[x]
			if q==nil{
				continue
			}
			p := convertedPixels[x][y]
			if p==nil{
				continue
			}
			original,ok := color.RGBAModel.Convert(p).(color.RGBA)
			if ok{
				nImg.Set(x,y,original)
			}
		}
	}
	converted := nImg.SubImage(rect)
	return converted, nil
}

func greyScale(pixels [][]color.Color) [][]color.Color{
	xLen := len(pixels)
	yLen := len(pixels[0])

	//create new image
	newImage:=make([][]color.Color, xLen)
	for i:=0;i<len(newImage);i++{
		newImage[i] = make([]color.Color,yLen)
	}

	//idea is processing pixels in parallel
	wg := sync.WaitGroup{}
	for x:=0;x<xLen;x++{
		for y:=0;y<yLen; y++{
			wg.Add(1)
			go func(x,y int) {
				pixel :=pixels[x][y]
				originalColor, ok := color.RGBAModel.Convert(pixel).(color.RGBA)
				if !ok{
					fmt.Println("snap something went wrong")
				}
				grey := uint8(float64(originalColor.R)*0.21 + float64(originalColor.G)*0.72 + float64(originalColor.B)*0.07)
				col :=color.RGBA{
					R: grey,
					G: grey,
					B: grey,
					A: originalColor.A,
				}
				newImage[x][y] = col
				wg.Done()
			}(x,y)

		}
	}
	wg.Wait()
	return newImage
}