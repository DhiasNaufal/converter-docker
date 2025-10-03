package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Version = "1.0.0"

// CityGMLMerger handles the merging of CityGML files
type CityGMLMerger struct {
	Debug bool
}

// Bounds represents a bounding box
type Bounds struct {
	LowerX       float64
	LowerY       float64
	LowerZ       float64
	UpperX       float64
	UpperY       float64
	UpperZ       float64
	SRS          string
	SRSDimension string
}

// XMLNode represents a generic XML node for manipulation
type XMLNode struct {
	XMLName xml.Name
	Attrs   []xml.Attr `xml:",any,attr"`
	Content string     `xml:",chardata"`
	Nodes   []XMLNode  `xml:",any"`
}

// NewCityGMLMerger creates a new merger instance
func NewCityGMLMerger(debug bool) *CityGMLMerger {
	return &CityGMLMerger{
		Debug: debug,
	}
}

// GetCityGMLFiles finds all CityGML files in the directory
func (c *CityGMLMerger) GetCityGMLFiles(directoryPath string) ([]string, error) {
	var files []string

	// Check if directory exists
	if _, err := os.Stat(directoryPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory not found: %s", directoryPath)
	}

	// Find .gml and .xml files
	gmlPattern := filepath.Join(directoryPath, "*.gml")
	xmlPattern := filepath.Join(directoryPath, "*.xml")

	gmlFiles, err := filepath.Glob(gmlPattern)
	if err != nil {
		return nil, err
	}

	xmlFiles, err := filepath.Glob(xmlPattern)
	if err != nil {
		return nil, err
	}

	files = append(files, gmlFiles...)
	files = append(files, xmlFiles...)

	if len(files) == 0 {
		return nil, fmt.Errorf("no CityGML files found in directory: %s", directoryPath)
	}

	sort.Strings(files)
	return files, nil
}

// ValidateCityGMLFile checks if the file is a valid CityGML file
func (c *CityGMLMerger) ValidateCityGMLFile(filePath string) bool {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		if c.Debug {
			fmt.Printf("Warning: Could not read file %s: %v\n", filePath, err)
		}
		return false
	}

	// Simple validation: check if it contains CityModel
	content := string(data)
	if strings.Contains(content, "CityModel") {
		return true
	}

	if c.Debug {
		fmt.Printf("Warning: File %s does not appear to be a CityGML file\n", filePath)
	}
	return false
}

// ExtractBounds extracts bounding box from XML content
func (c *CityGMLMerger) ExtractBounds(content string) *Bounds {
	// Simple regex-based extraction for bounds
	lowerCornerRegex := `<gml:lowerCorner[^>]*>([^<]+)</gml:lowerCorner>`
	upperCornerRegex := `<gml:upperCorner[^>]*>([^<]+)</gml:upperCorner>`
	srsRegex := `srsName="([^"]+)"`

	lowerMatch := findStringSubmatch(lowerCornerRegex, content)
	upperMatch := findStringSubmatch(upperCornerRegex, content)
	srsMatch := findStringSubmatch(srsRegex, content)

	if len(lowerMatch) < 2 || len(upperMatch) < 2 {
		return nil
	}

	lowerCoords := strings.Fields(strings.TrimSpace(lowerMatch[1]))
	upperCoords := strings.Fields(strings.TrimSpace(upperMatch[1]))

	if len(lowerCoords) < 3 || len(upperCoords) < 3 {
		return nil
	}

	lowerX, err1 := strconv.ParseFloat(lowerCoords[0], 64)
	lowerY, err2 := strconv.ParseFloat(lowerCoords[1], 64)
	lowerZ, err3 := strconv.ParseFloat(lowerCoords[2], 64)
	upperX, err4 := strconv.ParseFloat(upperCoords[0], 64)
	upperY, err5 := strconv.ParseFloat(upperCoords[1], 64)
	upperZ, err6 := strconv.ParseFloat(upperCoords[2], 64)

	if err1 != nil || err2 != nil || err3 != nil || err4 != nil || err5 != nil || err6 != nil {
		return nil
	}

	srs := ""
	if len(srsMatch) >= 2 {
		srs = srsMatch[1]
	}

	return &Bounds{
		LowerX:       lowerX,
		LowerY:       lowerY,
		LowerZ:       lowerZ,
		UpperX:       upperX,
		UpperY:       upperY,
		UpperZ:       upperZ,
		SRS:          srs,
		SRSDimension: "3",
	}
}

// CalculateMergedBounds calculates merged bounding box
func (c *CityGMLMerger) CalculateMergedBounds(boundsList []*Bounds) *Bounds {
	if len(boundsList) == 0 {
		return nil
	}

	merged := &Bounds{
		LowerX:       boundsList[0].LowerX,
		LowerY:       boundsList[0].LowerY,
		LowerZ:       boundsList[0].LowerZ,
		UpperX:       boundsList[0].UpperX,
		UpperY:       boundsList[0].UpperY,
		UpperZ:       boundsList[0].UpperZ,
		SRS:          boundsList[0].SRS,
		SRSDimension: "3",
	}

	for _, bounds := range boundsList[1:] {
		if bounds.LowerX < merged.LowerX {
			merged.LowerX = bounds.LowerX
		}
		if bounds.LowerY < merged.LowerY {
			merged.LowerY = bounds.LowerY
		}
		if bounds.LowerZ < merged.LowerZ {
			merged.LowerZ = bounds.LowerZ
		}
		if bounds.UpperX > merged.UpperX {
			merged.UpperX = bounds.UpperX
		}
		if bounds.UpperY > merged.UpperY {
			merged.UpperY = bounds.UpperY
		}
		if bounds.UpperZ > merged.UpperZ {
			merged.UpperZ = bounds.UpperZ
		}
	}

	return merged
}

// UpdateIDsWithPrefix updates all UUID_ prefixes with custom prefix
func (c *CityGMLMerger) UpdateIDsWithPrefix(content, prefix string) string {
	if c.Debug {
		fmt.Printf("  Updating IDs with prefix: %s\n", prefix)
	}

	// Replace gml:id="UUID_" with gml:id="prefix_"
	content = strings.ReplaceAll(content, `gml:id="UUID_`, `gml:id="`+prefix+`_`)
	content = strings.ReplaceAll(content, `id="UUID_`, `id="`+prefix+`_`)

	// Replace xlink:href="#UUID_" with xlink:href="#prefix_"
	content = strings.ReplaceAll(content, `xlink:href="#UUID_`, `xlink:href="#`+prefix+`_`)

	// Replace any other UUID_ references
	content = strings.ReplaceAll(content, `"UUID_`, `"`+prefix+`_`)

	return content
}

// UpdateDescriptions updates descriptions with author name
func (c *CityGMLMerger) UpdateDescriptions(content, authorName string) string {
	if c.Debug {
		fmt.Printf("  Updating descriptions with author: %s\n", authorName)
	}

	// Replace "created by converter" with "created by authorName"
	content = strings.ReplaceAll(content, "created by converter", "created by "+authorName)

	return content
}

// ExtractCityObjects extracts cityObjectMember elements from content
func (c *CityGMLMerger) ExtractCityObjects(content string) []string {
	var cityObjects []string

	// Find all cityObjectMember elements
	startTag := "<core:cityObjectMember>"
	endTag := "</core:cityObjectMember>"

	// Also try without namespace prefix
	if !strings.Contains(content, startTag) {
		startTag = "<cityObjectMember>"
		endTag = "</cityObjectMember>"
	}

	pos := 0
	for {
		start := strings.Index(content[pos:], startTag)
		if start == -1 {
			break
		}
		start += pos

		end := strings.Index(content[start:], endTag)
		if end == -1 {
			break
		}
		end += start + len(endTag)

		cityObject := content[start:end]
		cityObjects = append(cityObjects, cityObject)

		pos = end
	}

	return cityObjects
}

// ExtractRootAttributes extracts namespace declarations and attributes from the first file
func (c *CityGMLMerger) ExtractRootAttributes(filePaths []string) string {
	for _, filePath := range filePaths {
		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			continue
		}

		content := string(data)

		// Find the CityModel opening tag
		cityModelStart := strings.Index(content, "<")
		if cityModelStart == -1 {
			continue
		}

		cityModelEnd := strings.Index(content[cityModelStart:], ">")
		if cityModelEnd == -1 {
			continue
		}

		rootTag := content[cityModelStart : cityModelStart+cityModelEnd+1]

		// Extract just the attributes part
		if strings.Contains(rootTag, "CityModel") {
			return rootTag
		}
	}

	// Fallback: minimal CityModel tag
	return `<core:CityModel xmlns:core="http://www.opengis.net/citygml/2.0" xmlns:gml="http://www.opengis.net/gml" xmlns:bldg="http://www.opengis.net/citygml/building/2.0" xmlns:app="http://www.opengis.net/citygml/appearance/2.0" xmlns:gen="http://www.opengis.net/citygml/generics/2.0" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">`
}

// CreateMergedCityGML creates the merged CityGML content
func (c *CityGMLMerger) CreateMergedCityGML(filePaths []string, outputName, authorName string) (string, error) {
	var allBounds []*Bounds
	var allCityObjects []string

	fmt.Printf("Processing %d CityGML files...\n", len(filePaths))

	for i, filePath := range filePaths {
		if c.Debug {
			fmt.Printf("Processing file %d/%d: %s\n", i+1, len(filePaths), filepath.Base(filePath))
		}

		data, err := ioutil.ReadFile(filePath)
		if err != nil {
			fmt.Printf("Error reading %s: %v\n", filePath, err)
			continue
		}

		content := string(data)

		// Extract bounds
		bounds := c.ExtractBounds(content)
		if bounds != nil {
			allBounds = append(allBounds, bounds)
		}

		// Extract city objects
		cityObjects := c.ExtractCityObjects(content)

		// Process each city object
		for _, cityObject := range cityObjects {
			// Update IDs with prefix
			updatedObject := c.UpdateIDsWithPrefix(cityObject, outputName)

			// Update descriptions
			updatedObject = c.UpdateDescriptions(updatedObject, authorName)

			allCityObjects = append(allCityObjects, updatedObject)
		}

		if c.Debug {
			fmt.Printf("  Extracted %d city objects from %s\n", len(cityObjects), filepath.Base(filePath))
		}
	}

	// Get root attributes from first file
	rootTag := c.ExtractRootAttributes(filePaths)

	// Build merged CityGML
	var result strings.Builder

	// XML declaration and header
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	result.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	result.WriteString("\n<!-- Merged CityGML File -->")
	result.WriteString(fmt.Sprintf("\n<!-- Generated by CityGML Merger v%s on %s -->", Version, timestamp))
	result.WriteString("\n<!-- Original files merged into single CityGML document -->")
	result.WriteString(fmt.Sprintf("\n<!-- UUID_ prefixes replaced with %s_ -->", outputName))
	result.WriteString(fmt.Sprintf("\n<!-- Descriptions updated with author name: %s -->", authorName))
	result.WriteString("\n")

	// Root element
	result.WriteString(rootTag)
	result.WriteString("\n")

	// Name element
	result.WriteString(fmt.Sprintf("  <gml:name>%s</gml:name>\n", outputName))

	// Bounded by element
	if len(allBounds) > 0 {
		mergedBounds := c.CalculateMergedBounds(allBounds)
		if mergedBounds != nil {
			result.WriteString("  <gml:boundedBy>\n")
			result.WriteString(fmt.Sprintf("    <gml:Envelope srsName=\"%s\" srsDimension=\"3\">\n", mergedBounds.SRS))
			result.WriteString(fmt.Sprintf("      <gml:lowerCorner>%f %f %f</gml:lowerCorner>\n",
				mergedBounds.LowerX, mergedBounds.LowerY, mergedBounds.LowerZ))
			result.WriteString(fmt.Sprintf("      <gml:upperCorner>%f %f %f</gml:upperCorner>\n",
				mergedBounds.UpperX, mergedBounds.UpperY, mergedBounds.UpperZ))
			result.WriteString("    </gml:Envelope>\n")
			result.WriteString("  </gml:boundedBy>\n")
		}
	}

	// Add all city objects
	for _, cityObject := range allCityObjects {
		// Indent the city object
		lines := strings.Split(cityObject, "\n")
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				result.WriteString("  " + line + "\n")
			}
		}
	}

	// Close root element
	result.WriteString("</core:CityModel>\n")

	fmt.Printf("Successfully merged %d city objects from %d files\n", len(allCityObjects), len(filePaths))
	fmt.Printf("All UUID_ prefixes replaced with '%s_'\n", outputName)
	fmt.Printf("All descriptions updated with author name: '%s'\n", authorName)

	return result.String(), nil
}

// MergeFiles is the main method to merge CityGML files
func (c *CityGMLMerger) MergeFiles(inputDirectory, outputFile, outputName, authorName string) error {
	// Get all CityGML files
	filePaths, err := c.GetCityGMLFiles(inputDirectory)
	if err != nil {
		return err
	}

	if c.Debug {
		fmt.Printf("Found %d potential CityGML files\n", len(filePaths))
	}

	// Validate files
	var validFiles []string
	for _, filePath := range filePaths {
		if c.ValidateCityGMLFile(filePath) {
			validFiles = append(validFiles, filePath)
		} else if c.Debug {
			fmt.Printf("Skipping invalid CityGML file: %s\n", filePath)
		}
	}

	if len(validFiles) == 0 {
		return fmt.Errorf("no valid CityGML files found in the directory")
	}

	fmt.Printf("Processing %d valid CityGML files\n", len(validFiles))

	if c.Debug {
		fmt.Printf("Will replace 'UUID_' prefix with '%s_' in all IDs\n", outputName)
		fmt.Printf("Will replace 'created by converter' with 'created by %s' in descriptions\n", authorName)
	}

	// Create merged CityGML
	mergedContent, err := c.CreateMergedCityGML(validFiles, outputName, authorName)
	if err != nil {
		return err
	}

	// Write output file
	err = ioutil.WriteFile(outputFile, []byte(mergedContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file: %v", err)
	}

	fmt.Printf("Successfully created merged CityGML file: %s\n", outputFile)
	return nil
}

// Helper function to find regex matches (simplified)
func findStringSubmatch(pattern, text string) []string {
	// Simple string matching for the patterns we need
	if pattern == `<gml:lowerCorner[^>]*>([^<]+)</gml:lowerCorner>` {
		start := strings.Index(text, "<gml:lowerCorner")
		if start == -1 {
			return nil
		}
		contentStart := strings.Index(text[start:], ">")
		if contentStart == -1 {
			return nil
		}
		contentStart += start + 1

		end := strings.Index(text[contentStart:], "</gml:lowerCorner>")
		if end == -1 {
			return nil
		}
		end += contentStart

		return []string{text[start:end], text[contentStart:end]}
	}

	if pattern == `<gml:upperCorner[^>]*>([^<]+)</gml:upperCorner>` {
		start := strings.Index(text, "<gml:upperCorner")
		if start == -1 {
			return nil
		}
		contentStart := strings.Index(text[start:], ">")
		if contentStart == -1 {
			return nil
		}
		contentStart += start + 1

		end := strings.Index(text[contentStart:], "</gml:upperCorner>")
		if end == -1 {
			return nil
		}
		end += contentStart

		return []string{text[start:end], text[contentStart:end]}
	}

	if pattern == `srsName="([^"]+)"` {
		start := strings.Index(text, `srsName="`)
		if start == -1 {
			return nil
		}
		start += 9 // length of 'srsName="'

		end := strings.Index(text[start:], `"`)
		if end == -1 {
			return nil
		}
		end += start

		return []string{text[start-9 : end+1], text[start:end]}
	}

	return nil
}

func main() {
	var inputDir = flag.String("input", "", "Directory containing CityGML files to merge (required)")
	var outputFile = flag.String("output", "", "Output path for merged CityGML file (required)")
	var outputName = flag.String("name", "Merged_CityModel", "Name for the merged city model and prefix for building IDs")
	var authorName = flag.String("author", "Fairuz Akmal Pradana", "Author name to replace 'converter' in descriptions")
	var debug = flag.Bool("debug", false, "Enable debug output with detailed processing info")
	var help = flag.Bool("help", false, "Show help message")

	flag.Parse()

	if *help {
		fmt.Printf("CityGML Merger v%s\n", Version)
		fmt.Println("Merges multiple CityGML files from a directory into a single CityGML file")
		fmt.Println("\nUsage:")
		fmt.Printf("  %s --input <input_dir> --output <output_file> [options]\n\n", os.Args[0])
		fmt.Println("Required arguments:")
		fmt.Println("  --input      Directory containing CityGML files to merge")
		fmt.Println("  --output     Output path for merged CityGML file")
		fmt.Println("\nOptional arguments:")
		fmt.Println("  --name       Name for merged city model and ID prefix (default: Merged_CityModel)")
		fmt.Println("  --author     Author name to replace 'converter' in descriptions (default: Fairuz Akmal Pradana)")
		fmt.Println("  --debug      Enable debug output with detailed processing info")
		fmt.Println("  --help       Show this help message")
		fmt.Println("\nExamples:")
		fmt.Printf("  %s --input ./citygml_files --output merged_output.gml\n", os.Args[0])
		fmt.Printf("  %s --input ./input_folder --output ./output/merged_city.gml --name \"AG_09_C\"\n", os.Args[0])
		fmt.Printf("  %s --input ./input_folder --output ./output/merged_city.gml --name \"AG_09_C\" --author \"John Doe\"\n", os.Args[0])
		fmt.Println("\nThe script will:")
		fmt.Println("  1. Replace \"UUID_\" prefix in all building IDs with the --name parameter")
		fmt.Println("  2. Replace \"created by converter\" with \"created by [author]\" in all descriptions")
		fmt.Println("\nExamples of changes:")
		fmt.Println("  - UUID_d281adfc-4901-0f52-540b-48625 -> AG_09_C_d281adfc-4901-0f52-540b-48625")
		fmt.Println("  - \"10, created by converter\" -> \"10, created by Fairuz Akmal Pradana\"")
		os.Exit(0)
	}

	if *inputDir == "" || *outputFile == "" {
		fmt.Println("Error: --input and --output arguments are required")
		fmt.Println("Use --help for usage information")
		os.Exit(1)
	}

	// Validate input directory
	if info, err := os.Stat(*inputDir); err != nil {
		fmt.Printf("Error: Cannot access input directory '%s': %v\n", *inputDir, err)
		os.Exit(1)
	} else if !info.IsDir() {
		fmt.Printf("Error: Input path '%s' is not a directory\n", *inputDir)
		os.Exit(1)
	}

	// Convert paths to absolute
	absInputDir, err := filepath.Abs(*inputDir)
	if err != nil {
		fmt.Printf("Error: Invalid input directory '%s': %v\n", *inputDir, err)
		os.Exit(1)
	}

	absOutputFile, err := filepath.Abs(*outputFile)
	if err != nil {
		fmt.Printf("Error: Invalid output file '%s': %v\n", *outputFile, err)
		os.Exit(1)
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(absOutputFile)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		fmt.Printf("Error: Cannot create output directory '%s': %v\n", outputDir, err)
		os.Exit(1)
	}

	if *debug {
		fmt.Println("Debug mode enabled")
		fmt.Printf("Input Directory: %s\n", absInputDir)
		fmt.Printf("Output File: %s\n", absOutputFile)
		fmt.Printf("Output Name: %s\n", *outputName)
		fmt.Printf("Author Name: %s\n", *authorName)
	}

	fmt.Printf("CityGML Merger v%s\n", Version)
	fmt.Println("==================")

	// Create merger instance
	merger := NewCityGMLMerger(*debug)

	// Merge files
	if err := merger.MergeFiles(absInputDir, absOutputFile, *outputName, *authorName); err != nil {
		fmt.Printf("Error during merging process: %v\n", err)
		os.Exit(1)
	}
}
