package main

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/parser"
	"github.com/nfnt/resize"
	"golang.org/x/image/draw"
	"gopkg.in/yaml.v3"
)

type ContentYAML struct {
	Title string `yaml:"title"`
}

func getFileContents(filename string) []byte {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatal(err)
	}

	return bytes
}

// isDirectory determines if a file represented
// by `path` is a directory or not
func isDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}

	return fileInfo.IsDir(), err
}

func copy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func resizeJPEG(sourceFilename, destFilename string) {
	input, _ := os.Open(sourceFilename)
	defer input.Close()

	output, _ := os.Create(destFilename)
	defer output.Close()

	// Decode the image (from PNG to image.Image):
	src, _ := jpeg.Decode(input)

	// Max JPEG size = 1000x1000 bounding box.
	maxSize := 1000
	portrait := src.Bounds().Max.Y > src.Bounds().Max.X
	scaleRatio := 0.0
	if portrait {
		scaleRatio = float64(maxSize) / float64(src.Bounds().Max.Y)
	} else {
		scaleRatio = float64(maxSize) / float64(src.Bounds().Max.X)
	}
	newWidth := int(float64(src.Bounds().Max.X) * scaleRatio)
	newHeight := int(float64(src.Bounds().Max.Y) * scaleRatio)

	// Set the expected size that you want:
	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// Resize:
	draw.NearestNeighbor.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)
	newImage := resize.Resize(uint(newWidth), uint(newHeight), src, resize.Bicubic)
	// Encode to `output`:
	jpeg.Encode(output, newImage, nil)
}

func main() {
	start := time.Now()
	fmt.Println("Ich bin Klaus v0.2")
	markdownCount := 0
	otherFileCount := 0

	publishedFolder := "published"
	// Create the "published" folder if it doesn't exist.
	os.MkdirAll(publishedFolder, 0770)
	// Copy the main stylesheet to the "published" folder.
	targetCSSFilename := "published/main.css"
	copy("templates/main.css", targetCSSFilename)

	err := filepath.WalkDir("content",
		func(theFile string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if theFile != "content" {
				ext := path.Ext(theFile)

				if ext == ".md" { // A markdown file, so we need to convert it into HTML and put it in the main template.
					fmt.Printf("Processing file: %s\n", theFile)

					contentBytes := getFileContents(theFile)
					templateBytes := getFileContents("templates/main.html")

					var markdownBytes []byte
					// Extract YAML between '---' and '---'.
					if string(contentBytes[0:3]) != "---" {
						log.Fatal("Your markdown files must have a section separated by --- and ---. It must contain a title.")
					}
					re := regexp.MustCompile(`(?m)^---.*`)
					loc := re.FindIndex(contentBytes)
					yamlStart := loc[0] + 3
					everythingAfterYamlStart := contentBytes[yamlStart+3:]
					loc2 := re.FindIndex(everythingAfterYamlStart)
					yamlEnd := loc2[0] + 5
					yamlBytes := []byte(strings.Trim(string(contentBytes[yamlStart:yamlEnd]), "\n\r\t "))
					contentYaml := ContentYAML{}

					err := yaml.Unmarshal(yamlBytes, &contentYaml)
					if err != nil {
						log.Fatalf("error: %v", err)
					}
					markdownBytes = contentBytes[yamlEnd:]
					templateBytes = []byte(strings.ReplaceAll(string(templateBytes), string("{TITLE}"), contentYaml.Title))

					extensions := parser.CommonExtensions | parser.AutoHeadingIDs
					parser := parser.NewWithExtensions(extensions)
					cleansedLines := markdown.NormalizeNewlines(markdownBytes)
					markdownInHTML := markdown.ToHTML(cleansedLines, parser, nil)

					finalHTML := bytes.ReplaceAll(templateBytes, []byte("{BODY}"), markdownInHTML)

					targetFilename := theFile[len("content")+1:len(theFile)-len(ext)] + ".html"
					targetFilename = "published/" + targetFilename
					// Make sure target folder exists.
					os.MkdirAll(filepath.Dir(targetFilename), 0770)

					if err := os.WriteFile(targetFilename, finalHTML, 0666); err != nil {
						log.Fatal(err)
					}

					markdownCount++
				} else if strings.ToLower(ext) == ".jpg" {
					targetFilename := "published/" + theFile[len("content")+1:]
					resizeJPEG(theFile, targetFilename)
				} else { // Not a .md file, so just copy it to the published folder.
					targetFilename := "published/" + theFile[len("content")+1:]
					// if folder - create it.
					isDir, err := isDirectory(theFile)
					if err != nil {
						log.Fatal("Error checking directory " + err.Error())
					}
					if isDir {
						os.MkdirAll(targetFilename, 0770)
					}
					// if file - copy it.

					copy(theFile, targetFilename)
					otherFileCount++
				}

				// In future, I want to resize JPGs automatically if they're too big.
			}

			return nil
		})
	if err != nil {
		log.Println(err)
	}

	fmt.Printf("Publish complete. Published %d markdown files and %d other files. Time: %v\n", markdownCount, otherFileCount, time.Since(start))
}
