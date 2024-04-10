package helpers

import (
	"fmt"
	"regexp"
)

func CutImageData(content string) (string, string) {
	re := regexp.MustCompile(`\{.*?"@context".*?\}`)
	matchContent := re.FindString(content)
	fmt.Println(matchContent)
	cleanedContent := re.ReplaceAllString(content, "")
	fmt.Println(cleanedContent)
	return cleanedContent, matchContent
}
