package main

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"strconv"
	"strings"
)

// Find out if two image matrices are identical.  If diff is set, create a
// matrix of locations where there are differences.
func equalImgMatrix(mat1 [][]byte, mat2 [][]byte, diff bool) (bool, [][]bool, error) {

	// First, quick check with hashes
	sha1, err := hash(mat1)
	if err != nil {
		return false, nil, err
	}
	sha2, err := hash(mat2)
	if err != nil {
		return false, nil, err
	}

	if bytes.Equal(sha1, sha2) {
		return true, nil, nil
	}

	if diff {
		fmt.Fprintf(os.Stderr, "generating difference files for matrices %dx%d\n", len(mat1), len(mat1[0]))
		diff, err := diffMatrix(mat1, mat2)
		fmt.Fprintf(os.Stderr, "received difference matrix %dx%d\n", len(diff), len(diff[0]))
		if err != nil {
			return false, nil, err
		}

		return false, diff, nil
	}

	return false, nil, nil
}

// Given two RGB matrices, return a matrix that is true for every different pixel
func diffMatrix(mat1 [][]byte, mat2 [][]byte) ([][]bool, error) {
	if len(mat1) != len(mat2) {
		return nil, errors.New("diffMatrix: inputs do not have the same height")
	}

	diff := make([][]bool, len(mat1))
	for y := range mat1 {
		if len(mat1[y]) != len(mat2[y]) {
			return nil, errors.New("diffMatrix: inputs do not have the same width at row " + strconv.Itoa(y))
		}
		diff[y] = make([]bool, len(mat1[y])/3)
		for x := range len(mat1[y]) / 3 {
			diff[y][x] = !bytes.Equal(mat1[y][x*3:(x+1)*3], mat2[y][x*3:(x+1)*3])
		}
	}
	return diff, nil
}

// Compute the sha256 hash for a 2d byte matrix
func hash(mat [][]byte) ([]byte, error) {
	h := sha256.New()
	for y := range mat {
		_, err := h.Write(mat[y])
		if err != nil {
			return nil, err
		}
	}
	return h.Sum(nil), nil
}

// Read a PPM file into a 2D byte matrix
func ppmToMatrix(rd io.Reader) ([][]byte, error) {
	reader := bufio.NewReader(rd)

	// Parse header
	format, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	format = strings.TrimSpace(format)
	var isBinary bool
	if format == "P3" {
		isBinary = false
	} else if format == "P6" {
		isBinary = true
	} else {
		return nil, fmt.Errorf("unsupported PPM format: %s", format)
	}

	sizeStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	sizeParts := strings.Split(strings.TrimSpace(sizeStr), " ")
	if len(sizeParts) != 2 {
		return nil, fmt.Errorf("invalid size format: %s", sizeStr)
	}
	width, err := strconv.Atoi(sizeParts[0])
	if err != nil {
		return nil, err
	}
	height, err := strconv.Atoi(sizeParts[1])
	if err != nil {
		return nil, err
	}

	maxColorStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, err
	}
	maxColor, err := strconv.Atoi(strings.TrimSpace(maxColorStr))
	if err != nil {
		return nil, err
	}

	// Parse pixel data
	fmt.Fprintf(os.Stderr, "parsing pixel data, width=%d, height=%d, maxColor=%d, isBinary=%t\n", width, height, maxColor, isBinary)
	pixels := make([][]byte, height)
	for y := range height {
		pixels[y] = make([]byte, width*3)
		for x := range width {
			for i := range 3 {
				var color int
				if isBinary {
					var b byte
					err = binary.Read(reader, binary.BigEndian, &b)
					if err != nil {
						return nil, err
					}
					color = int(b)
				} else {
					a, err := readNextValue(reader)
					if err != nil {
						return nil, err
					}
					color, err = strconv.Atoi(a)
					if err != nil {
						return nil, err
					}
				}
				pixels[y][x*3+i] = byte(color * 255 / maxColor)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "finished parsing pixel data\n")

	return pixels, nil
}

// Used in reading PPMs
func readNextValue(reader *bufio.Reader) (string, error) {
	var value string
	for {
		char, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				// If we've read some value before EOF, return it.
				if value != "" {
					return value, nil
				}
				return "", err // Return EOF if no value was read
			}
			return "", err // Return any other error
		}
		if char == ' ' || char == '\n' || char == '\t' {
			if value != "" {
				return value, nil
			}
			// Skip leading whitespace
		} else {
			value += string(char)
		}
	}
}

// Create a 2D byte matrix that depicts a circle, with values 0 and 255.
func circle(radius int) [][]byte {
	size := 2*radius + 1
	stamp := make([][]byte, size)
	for i := range stamp {
		stamp[i] = make([]byte, size)
	}

	centerY := radius
	centerX := radius
	radiusSquared := radius * radius

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := x - centerX
			dy := y - centerY
			if dx*dx+dy*dy <= radiusSquared {
				stamp[y][x] = 255
			}
		}
	}
	return stamp
}

// Given a 2D byte matrix and a matrix of locations where it is to be
// marked, highlight a circle of the given radius at each location.
func diffImage(mat [][]byte, diff [][]bool, radius int) [][]byte {

	newMat := make([][]byte, len(mat))
	for y := range mat {
		newMat[y] = make([]byte, len(mat[y]))
		copy(newMat[y], mat[y])
	}

	stamp := circle(radius)
	for y := range diff {
		for x := range diff[y] {
			if diff[y][x] {
				highlightStamp(mat, newMat, stamp, x*3, y)
			}
		}
	}
	return newMat
}

// Adds the highlight stamp into newImage, which should start out as
// a copy of img since we do not want to double, triple, etc the effect
func highlightStamp(img, newImage, stamp [][]byte, centerX, centerY int) {
	stampRows := len(stamp)
	stampCols := len(stamp[0])
	rows := len(img)
	cols := len(img[0])

	for y := range stampRows {
		matrixY := centerY - len(stamp)/2 + y
		if matrixY < 0 || matrixY >= rows {
			continue
		}
		for x := range stampCols {
			matrixX := centerX - len(stamp[0])/2*3 + x*3
			if matrixX < 0 || matrixX+2 >= cols {
				continue
			}
			if stamp[y][x] == 0 {
				continue
			}
			newImage[matrixY][matrixX], newImage[matrixY][matrixX+1], newImage[matrixY][matrixX+2] =
				highlightPixel(img[matrixY][matrixX], img[matrixY][matrixX+1], img[matrixY][matrixX+2])
		}
	}
}

// Convert a 2D RGB byte matrix to a PNG Image.
func rgbToPNG(matrix [][]byte) image.Image {
	height := len(matrix)
	width := len(matrix[0]) / 3

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := range height {
		for x := range width {
			r := matrix[y][x*3]
			g := matrix[y][x*3+1]
			b := matrix[y][x*3+2]
			img.Set(x, y, color.RGBA{r, g, b, 255}) // Assuming full opacity (alpha=255)
		}
	}
	return img
}

// Add a yellow highlight to a single pixel, by blending with pure yellow
func highlightPixel(r, g, b byte) (byte, byte, byte) {
	blendFactor := 0.5

	red := float64(r)*(1-blendFactor) + float64(255)*blendFactor
	green := float64(g)*(1-blendFactor) + float64(255)*blendFactor
	blue := float64(b) * (1 - blendFactor)

	if red > 255 {
		red = 255
	}
	if green > 255 {
		green = 255
	}
	if blue > 255 {
		blue = 255
	}

	return byte(red), byte(green), byte(blue)
}

// Join two 2D matrices side-by-side, separating with a black line with width padding
func joinImages(img1, img2 [][]byte, padding int) [][]byte {
	height := len(img1)
	width := len(img1[0]) + padding + len(img2[0])
	newImg := make([][]byte, height)
	for i := range newImg {
		newImg[i] = make([]byte, width)
		copy(newImg[i], img1[i])
		// padding should be initialized to 0, so black
		copy(newImg[i][len(img1[i])+padding+1:], img2[i])
	}
	return newImg
}
