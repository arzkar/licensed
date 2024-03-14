package main

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/pflag"
)

var (
	licenseName     string
	userName        string
	year            string
	listLicenses    bool
	projectDir      string
	ignoredPatterns []string

	//go:embed .licensed-ignore
	licensedIgnoreFile embed.FS

	//go:embed comment-syntax.txt
	commentSyntaxFile embed.FS
)

func shouldIgnoreFile(filePath string) bool {
	for _, pattern := range ignoredPatterns {
		matched, err := filepath.Match(pattern, filepath.Base(filePath))
		if err != nil {
			fmt.Printf("Error matching pattern %s: %s\n", pattern, err)
			continue
		}
		if matched {
			return true
		}
	}
	return false
}

func mergeFiles(embeddedFile, externalFile []byte) []byte {
	// Convert embedded file to set
	embeddedSet := make(map[string]struct{})
	lines := strings.Split(string(embeddedFile), "\n")
	for _, line := range lines {
		embeddedSet[line] = struct{}{}
	}

	// Merge external file into set
	externalLines := strings.Split(string(externalFile), "\n")
	for _, line := range externalLines {
		embeddedSet[line] = struct{}{}
	}

	// Convert set back to slice
	var merged []string
	for line := range embeddedSet {
		merged = append(merged, line)
	}

	// Join lines and return as byte slice
	return []byte(strings.Join(merged, "\n"))
}

func init() {
	pflag.StringVarP(&licenseName, "license", "l", "", "license name")
	pflag.StringVarP(&userName, "name", "n", "", "user name")
	pflag.StringVarP(&year, "year", "y", "", "year")
	pflag.BoolVar(&listLicenses, "list", false, "list all supported licenses")
	pflag.StringVar(&projectDir, "dir", ".", "path to the project directory")
	pflag.Parse()

	// Read the .licensed-ignore file from the project directory if present
	externalIgnoreFilePath := filepath.Join(projectDir, ".licensed-ignore")
	externalIgnoreFile, err := os.ReadFile(externalIgnoreFilePath)
	if err == nil {
		// Merge external file with embedded file
		licensedIgnoreFile = embed.FS(mergeFiles(licensedIgnoreFile, externalIgnoreFile))
	}

	// Read the comment-syntax.txt file from the project directory if present
	externalCommentFilePath := filepath.Join(projectDir, "comment-syntax.txt")
	externalCommentFile, err := os.ReadFile(externalCommentFilePath)
	if err == nil {
		// Merge external file with embedded file
		commentSyntaxFile = embed.FS(mergeFiles(commentSyntaxFile, externalCommentFile))
	}
}

func main() {
	if listLicenses {
		fetchLicenses()
	}

	if licenseName == "" || userName == "" || year == "" || projectDir == "" {
		pflag.PrintDefaults()
		os.Exit(1)
	}

	// Read the license file content
	licenseContent, err := os.ReadFile("licenses/" + licenseName + ".txt")
	if err != nil {
		fmt.Printf("Failed to read license file: %s\n", err)
		os.Exit(1)
	}

	// Modify the license content to include user name and year
	modifiedLicense := strings.ReplaceAll(string(licenseContent), "[year]", year)
	modifiedLicense = strings.ReplaceAll(modifiedLicense, "[fullname]", userName)

	// Split the .licensed-ignore file into patterns
	ignorePatterns := strings.Split(string(licensedIgnoreFile), "\n")
	for i := range ignorePatterns {
		ignorePatterns[i] = strings.TrimSpace(ignorePatterns[i])
	}

	// Set the ignoredPatterns
	ignoredPatterns = ignorePatterns

	// Recursively traverse the project directory
	err = filepath.Walk(projectDir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		// Check if the file should be ignored
		if shouldIgnoreFile(filePath) {
			return nil
		}

		// Determine the comment syntax based on the file extension
		fileExt := filepath.Ext(filePath)
		var commentSyntax string
		switch fileExt {
		case ".go", ".c", ".cpp":
			commentSyntax = "//"
		case ".java", ".js", ".ts", ".csharp":
			commentSyntax = "//"
		case ".py", ".rb":
			commentSyntax = "#"
		default:
			commentSyntax = "//"
		}

		// Add the modified license header to each file
		fmt.Printf("Adding modified license header to %s\n", filePath)
		return AddLicenseHeader(filePath, modifiedLicense, commentSyntax, userName, year)
	})
	if err != nil {
		fmt.Printf("Error traversing directory: %s\n", err)
		os.Exit(1)
	}

	// Write the license content to license.txt
	err = os.WriteFile("license.txt", []byte(modifiedLicense), 0644)
	if err != nil {
		fmt.Printf("Error writing license.txt: %s\n", err)
	}

	fmt.Println("License headers added successfully.")
}

func fetchLicenses() {
	// List all supported licenses
	fmt.Println("Supported licenses:")
	files, err := os.ReadDir("licenses")
	if err != nil {
		fmt.Printf("Failed to list licenses: %s\n", err)
		os.Exit(1)
	}
	for _, file := range files {
		fmt.Println("-", strings.TrimSuffix(file.Name(), filepath.Ext(file.Name())))
	}
	os.Exit(0)
}

func AddLicenseHeader(filePath, licenseContent, commentSyntax, userName, year string) error {
	// Read the existing file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Split the content into lines
	lines := strings.Split(string(content), "\n")

	// Check if the header already exists and update the name and year if necessary
	var headerExists bool
	for i, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), commentSyntax+" "+licenseContent) {
			headerExists = true
			// Check if name and year need to be updated
			if strings.Contains(line, "[fullname]") {
				lines[i] = strings.ReplaceAll(line, "[fullname]", userName)
			}
			if strings.Contains(line, "[year]") {
				lines[i] = strings.ReplaceAll(line, "[year]", year)
			}
			break
		}
	}

	// If the header doesn't exist, prompt the user to replace it
	if !headerExists {
		replace := true
		for _, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), commentSyntax) {
				replacePrompt := fmt.Sprintf("A different license header is detected in %s. Do you want to replace it? (y/n): ", filePath)
				fmt.Print(replacePrompt)
				var input string
				fmt.Scanln(&input)
				if strings.ToLower(strings.TrimSpace(input)) != "y" {
					replace = false
					break
				}
				break
			}
		}

		if replace {
			// Prepend the license header
			var newLines []string
			newLines = append(newLines, commentSyntax+" "+licenseContent)
			newLines = append(newLines, "")
			newLines = append(newLines, lines...)

			// Update the content with the new header
			lines = newLines
		}
	}

	// Join the lines back into content
	newContent := strings.Join(lines, "\n")

	// Write the new content back to the file
	err = os.WriteFile(filePath, []byte(newContent), 0644)
	if err != nil {
		return err
	}

	return nil
}
