package helpers

import (
	"regexp"
)

func CutImageData(content string) (string, string) {
	re := regexp.MustCompile(`\{.*?"@context".*?\}`)
	matchContent := re.FindString(content)
	cleanedContent := re.ReplaceAllString(content, "")
	return cleanedContent, matchContent
}
