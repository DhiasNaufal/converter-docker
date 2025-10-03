package main

import (
	"bufio"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unsafe"
)

/*
#cgo pkg-config: gdal
#include "gdal.h"
#include "gdal_alg.h"
#include "cpl_conv.h"
#include <stdlib.h>
*/
import "C"

const Version = "1.0.0"

// Vector3 represents a 3D vector
type Vector3 struct {
	X, Y, Z float64
}

// DTMData holds Digital Terrain Model information
type DTMData struct {
	Dataset      C.GDALDatasetH
	GeoTransform [6]float64
	Width        int
	Height       int
	NoDataValue  float64
	HasNoData    bool
}

// Statistics holds processing statistics
type Statistics struct {
	ProcessedFiles int
	FailedFiles    []FailedFile
	ElevationStats ElevationStats
}

// ElevationStats tracks elevation adjustments
type ElevationStats struct {
	TotalAdjustments int
	MinAdjustment    float64
	MaxAdjustment    float64
	AvgAdjustment    float64
	TotalAdjustment  float64
}

// FailedFile represents a failed file with error message
type FailedFile struct {
	Name  string
	Error string
}

// DTMElevator handles DTM-based elevation adjustments
type DTMElevator struct {
	InputDir  string
	OutputDir string
	DTMPath   string
	DTMData   *DTMData
	Stats     Statistics
	StartTime time.Time
	Debug     bool
}

// NewDTMElevator creates a new DTMElevator
func NewDTMElevator(inputDir, outputDir, dtmPath string, debug bool) *DTMElevator {
	return &DTMElevator{
		InputDir:  inputDir,
		OutputDir: outputDir,
		DTMPath:   dtmPath,
		Debug:     debug,
		StartTime: time.Now(),
		Stats: Statistics{
			ElevationStats: ElevationStats{
				MinAdjustment: math.Inf(1),
				MaxAdjustment: math.Inf(-1),
			},
		},
	}
}

// LoadDTM loads the DTM data from TIF file
func (de *DTMElevator) LoadDTM() error {
	fmt.Println("Loading DTM data...")

	// Register GDAL drivers
	C.GDALAllRegister()

	// Convert Go string to C string
	cPath := C.CString(de.DTMPath)
	defer C.free(unsafe.Pointer(cPath))

	// Open the DTM file
	dataset := C.GDALOpen(cPath, C.GA_ReadOnly)
	if dataset == nil {
		return fmt.Errorf("failed to open DTM file: %s", de.DTMPath)
	}

	// Get raster information
	width := int(C.GDALGetRasterXSize(dataset))
	height := int(C.GDALGetRasterYSize(dataset))

	// Get geotransform
	var geoTransform [6]C.double
	if C.GDALGetGeoTransform(dataset, &geoTransform[0]) != C.CE_None {
		C.GDALClose(dataset)
		return fmt.Errorf("failed to get geotransform from DTM")
	}

	// Convert C array to Go array
	var goGeoTransform [6]float64
	for i := 0; i < 6; i++ {
		goGeoTransform[i] = float64(geoTransform[i])
	}

	// Get the first band (elevation data)
	band := C.GDALGetRasterBand(dataset, 1)
	if band == nil {
		C.GDALClose(dataset)
		return fmt.Errorf("failed to get raster band from DTM")
	}

	// Get NoData value
	var hasNoData C.int
	noDataValue := float64(C.GDALGetRasterNoDataValue(band, &hasNoData))

	de.DTMData = &DTMData{
		Dataset:      dataset,
		GeoTransform: goGeoTransform,
		Width:        width,
		Height:       height,
		NoDataValue:  noDataValue,
		HasNoData:    hasNoData != 0,
	}

	fmt.Printf("DTM loaded successfully:\n")
	fmt.Printf("  Dimensions: %dx%d pixels\n", width, height)
	fmt.Printf("  Origin: (%.6f, %.6f)\n", goGeoTransform[0], goGeoTransform[3])
	fmt.Printf("  Pixel size: (%.6f, %.6f)\n", goGeoTransform[1], goGeoTransform[5])
	if hasNoData != 0 {
		fmt.Printf("  NoData value: %.6f\n", noDataValue)
	}

	return nil
}

// CloseDTM closes the DTM dataset
func (de *DTMElevator) CloseDTM() {
	if de.DTMData != nil && de.DTMData.Dataset != nil {
		C.GDALClose(de.DTMData.Dataset)
	}
}

// GetElevationAtPoint gets elevation from DTM at given X,Y coordinates
func (de *DTMElevator) GetElevationAtPoint(x, y float64) (float64, error) {
	if de.DTMData == nil {
		return 0, fmt.Errorf("DTM data not loaded")
	}

	// Convert world coordinates to pixel coordinates using inverse geotransform
	gt := de.DTMData.GeoTransform

	// Inverse geotransform calculation
	det := gt[1]*gt[5] - gt[2]*gt[4]
	if det == 0 {
		return 0, fmt.Errorf("invalid geotransform matrix")
	}

	px := ((x-gt[0])*gt[5] - (y-gt[3])*gt[2]) / det
	py := ((y-gt[3])*gt[1] - (x-gt[0])*gt[4]) / det

	// Convert to integer pixel coordinates
	pixelX := int(math.Floor(px))
	pixelY := int(math.Floor(py))

	// Check bounds
	if pixelX < 0 || pixelX >= de.DTMData.Width || pixelY < 0 || pixelY >= de.DTMData.Height {
		return 0, fmt.Errorf("coordinates (%.6f, %.6f) are outside DTM bounds", x, y)
	}

	// Get the raster band
	band := C.GDALGetRasterBand(de.DTMData.Dataset, 1)
	if band == nil {
		return 0, fmt.Errorf("failed to get raster band")
	}

	// Read elevation value at pixel
	var buffer C.double
	err := C.GDALRasterIO(band, C.GF_Read, C.int(pixelX), C.int(pixelY), 1, 1,
		unsafe.Pointer(&buffer), 1, 1, C.GDT_Float64, 0, 0)
	if err != C.CE_None {
		return 0, fmt.Errorf("failed to read elevation data")
	}

	elevation := float64(buffer)

	// Check for NoData value
	if de.DTMData.HasNoData && elevation == de.DTMData.NoDataValue {
		return 0, fmt.Errorf("no elevation data available at coordinates (%.6f, %.6f)", x, y)
	}

	return elevation, nil
}

// GetElevationAtPointBilinear gets elevation using bilinear interpolation
func (de *DTMElevator) GetElevationAtPointBilinear(x, y float64) (float64, error) {
	if de.DTMData == nil {
		return 0, fmt.Errorf("DTM data not loaded")
	}

	// Convert world coordinates to pixel coordinates
	gt := de.DTMData.GeoTransform
	det := gt[1]*gt[5] - gt[2]*gt[4]
	if det == 0 {
		return 0, fmt.Errorf("invalid geotransform matrix")
	}

	px := ((x-gt[0])*gt[5] - (y-gt[3])*gt[2]) / det
	py := ((y-gt[3])*gt[1] - (x-gt[0])*gt[4]) / det

	// Get the four surrounding pixels
	x1 := int(math.Floor(px))
	y1 := int(math.Floor(py))
	x2 := x1 + 1
	y2 := y1 + 1

	// Check bounds
	if x1 < 0 || x2 >= de.DTMData.Width || y1 < 0 || y2 >= de.DTMData.Height {
		// Fall back to nearest neighbor if out of bounds
		return de.GetElevationAtPoint(x, y)
	}

	// Get fractional parts
	fx := px - float64(x1)
	fy := py - float64(y1)

	// Get the raster band
	band := C.GDALGetRasterBand(de.DTMData.Dataset, 1)
	if band == nil {
		return 0, fmt.Errorf("failed to get raster band")
	}

	// Read 2x2 pixel block
	buffer := make([]C.double, 4)
	err := C.GDALRasterIO(band, C.GF_Read, C.int(x1), C.int(y1), 2, 2,
		unsafe.Pointer(&buffer[0]), 2, 2, C.GDT_Float64, 0, 0)
	if err != C.CE_None {
		return 0, fmt.Errorf("failed to read elevation data")
	}

	// Check for NoData values
	if de.DTMData.HasNoData {
		for _, val := range buffer {
			if float64(val) == de.DTMData.NoDataValue {
				// Fall back to nearest neighbor if any NoData found
				return de.GetElevationAtPoint(x, y)
			}
		}
	}

	// Bilinear interpolation
	// buffer layout: [top-left, top-right, bottom-left, bottom-right]
	topLeft := float64(buffer[0])
	topRight := float64(buffer[1])
	bottomLeft := float64(buffer[2])
	bottomRight := float64(buffer[3])

	// Interpolate along X axis
	top := topLeft*(1-fx) + topRight*fx
	bottom := bottomLeft*(1-fx) + bottomRight*fx

	// Interpolate along Y axis
	elevation := top*(1-fy) + bottom*fy

	return elevation, nil
}

// LoadObjFile loads vertices and other data from OBJ file
func (de *DTMElevator) LoadObjFile(objPath string) ([]Vector3, []string, error) {
	file, err := os.Open(objPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var vertices []Vector3
	var allLines []string

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		allLines = append(allLines, line)

		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "v ") {
			// Parse vertex
			parts := strings.Fields(trimmedLine)
			if len(parts) >= 4 {
				x, err1 := strconv.ParseFloat(parts[1], 64)
				y, err2 := strconv.ParseFloat(parts[2], 64)
				z, err3 := strconv.ParseFloat(parts[3], 64)
				if err1 == nil && err2 == nil && err3 == nil {
					vertices = append(vertices, Vector3{x, y, z})
				} else {
					if de.Debug {
						fmt.Printf("Warning: Invalid vertex at line %d in %s: %s\n", lineNum, filepath.Base(objPath), line)
					}
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, nil, fmt.Errorf("error reading file: %v", err)
	}

	if len(vertices) == 0 {
		return nil, nil, fmt.Errorf("no valid vertices found")
	}

	return vertices, allLines, nil
}

// CalculateElevationAdjustment calculates how much to adjust Z coordinates
func (de *DTMElevator) CalculateElevationAdjustment(vertices []Vector3) (float64, error) {
	if len(vertices) == 0 {
		return 0, fmt.Errorf("no vertices to process")
	}

	// Find the minimum Z coordinate (bottom of the object)
	minZ := vertices[0].Z
	for _, vertex := range vertices {
		if vertex.Z < minZ {
			minZ = vertex.Z
		}
	}

	// Find vertices at or near the minimum Z (bottom vertices)
	tolerance := 0.01 // 1cm tolerance
	var bottomVertices []Vector3
	for _, vertex := range vertices {
		if math.Abs(vertex.Z-minZ) <= tolerance {
			bottomVertices = append(bottomVertices, vertex)
		}
	}

	if len(bottomVertices) == 0 {
		return 0, fmt.Errorf("no bottom vertices found")
	}

	// Sample DTM elevations at bottom vertex locations
	var elevations []float64
	validElevations := 0

	for _, vertex := range bottomVertices {
		elevation, err := de.GetElevationAtPointBilinear(vertex.X, vertex.Y)
		if err != nil {
			if de.Debug {
				fmt.Printf("    Warning: Could not get elevation at (%.6f, %.6f): %v\n", vertex.X, vertex.Y, err)
			}
			continue
		}
		elevations = append(elevations, elevation)
		validElevations++
	}

	if validElevations == 0 {
		return 0, fmt.Errorf("could not get DTM elevation for any bottom vertices")
	}

	// Calculate target elevation (average of valid DTM elevations)
	var totalElevation float64
	for _, elevation := range elevations {
		totalElevation += elevation
	}
	targetElevation := totalElevation / float64(validElevations)

	// Calculate adjustment needed
	adjustment := targetElevation - minZ

	if de.Debug {
		fmt.Printf("    Bottom vertices: %d (%.6f tolerance)\n", len(bottomVertices))
		fmt.Printf("    Valid DTM samples: %d\n", validElevations)
		fmt.Printf("    Current min Z: %.6f\n", minZ)
		fmt.Printf("    Target elevation: %.6f\n", targetElevation)
		fmt.Printf("    Adjustment: %.6f\n", adjustment)
	}

	return adjustment, nil
}

// AdjustVertices applies elevation adjustment to all vertices
func (de *DTMElevator) AdjustVertices(vertices []Vector3, adjustment float64) []Vector3 {
	adjustedVertices := make([]Vector3, len(vertices))
	for i, vertex := range vertices {
		adjustedVertices[i] = Vector3{
			X: vertex.X,
			Y: vertex.Y,
			Z: vertex.Z + adjustment,
		}
	}
	return adjustedVertices
}

// SaveObjFile saves the adjusted OBJ file
func (de *DTMElevator) SaveObjFile(outputPath string, adjustedVertices []Vector3, allLines []string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("# Elevated by DTM Elevator v%s\n", Version))
	writer.WriteString(fmt.Sprintf("# Original vertices adjusted based on DTM: %s\n", filepath.Base(de.DTMPath)))
	writer.WriteString(fmt.Sprintf("# Vertices: %d\n", len(adjustedVertices)))
	writer.WriteString("\n")

	vertexIndex := 0

	// Process each line from the original file
	for _, line := range allLines {
		trimmedLine := strings.TrimSpace(line)

		if strings.HasPrefix(trimmedLine, "v ") {
			// This is a vertex line - replace with adjusted vertex
			if vertexIndex < len(adjustedVertices) {
				vertex := adjustedVertices[vertexIndex]
				writer.WriteString(fmt.Sprintf("v %.6f %.6f %.6f\n", vertex.X, vertex.Y, vertex.Z))
				vertexIndex++
			} else {
				// Fallback: write original line if we somehow have more vertex lines than vertices
				writer.WriteString(line + "\n")
			}
		} else {
			// Write all non-vertex lines as-is (faces, normals, textures, etc.)
			writer.WriteString(line + "\n")
		}
	}

	if de.Debug {
		fmt.Printf("    Written %d vertices and %d total lines\n", vertexIndex, len(allLines))
	}

	return nil
}

// ProcessObjFile processes a single OBJ file
func (de *DTMElevator) ProcessObjFile(objPath string) {
	if de.Debug {
		fmt.Printf("\nProcessing: %s\n", filepath.Base(objPath))
	}

	// Load OBJ file
	if de.Debug {
		fmt.Println("  Loading OBJ data...")
	}
	vertices, allLines, err := de.LoadObjFile(objPath)
	if err != nil {
		fmt.Printf("  Failed to load OBJ file: %v\n", err)
		de.Stats.FailedFiles = append(de.Stats.FailedFiles, FailedFile{filepath.Base(objPath), err.Error()})
		return
	}

	if de.Debug {
		fmt.Printf("  Loaded %d vertices from %d lines\n", len(vertices), len(allLines))
	}

	// Calculate elevation adjustment
	if de.Debug {
		fmt.Println("  Calculating elevation adjustment...")
	}
	adjustment, err := de.CalculateElevationAdjustment(vertices)
	if err != nil {
		fmt.Printf("  Failed to calculate elevation adjustment: %v\n", err)
		de.Stats.FailedFiles = append(de.Stats.FailedFiles, FailedFile{filepath.Base(objPath), err.Error()})
		return
	}

	if de.Debug {
		fmt.Printf("  Elevation adjustment: %.6f meters\n", adjustment)
	}

	// Apply adjustment
	if de.Debug {
		fmt.Println("  Applying elevation adjustment...")
	}
	adjustedVertices := de.AdjustVertices(vertices, adjustment)

	// Save adjusted OBJ file
	baseName := filepath.Base(objPath)
	outputPath := filepath.Join(de.OutputDir, baseName)

	if de.Debug {
		fmt.Printf("  Saving to: %s\n", outputPath)
	}
	if err := de.SaveObjFile(outputPath, adjustedVertices, allLines); err != nil {
		fmt.Printf("  Failed to save adjusted OBJ file: %v\n", err)
		de.Stats.FailedFiles = append(de.Stats.FailedFiles, FailedFile{filepath.Base(objPath), err.Error()})
		return
	}

	// Update statistics
	de.Stats.ProcessedFiles++
	de.Stats.ElevationStats.TotalAdjustments++
	de.Stats.ElevationStats.TotalAdjustment += adjustment

	if adjustment < de.Stats.ElevationStats.MinAdjustment {
		de.Stats.ElevationStats.MinAdjustment = adjustment
	}
	if adjustment > de.Stats.ElevationStats.MaxAdjustment {
		de.Stats.ElevationStats.MaxAdjustment = adjustment
	}

	if de.Debug {
		fmt.Printf("  Successfully processed %s\n", filepath.Base(objPath))
	}
}

// ProcessAllFiles processes all OBJ files in the input directory
func (de *DTMElevator) ProcessAllFiles() error {
	// Ensure output directory exists
	if err := os.MkdirAll(de.OutputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Find all OBJ files
	pattern := filepath.Join(de.InputDir, "*.obj")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("error finding OBJ files: %v", err)
	}

	if len(matches) == 0 {
		fmt.Printf("No OBJ files found in directory: %s\n", de.InputDir)
		return nil
	}

	fmt.Printf("Found %d OBJ files to process\n", len(matches))
	fmt.Printf("Input directory: %s\n", de.InputDir)
	fmt.Printf("Output directory: %s\n", de.OutputDir)

	// Process each file
	for _, objPath := range matches {
		de.ProcessObjFile(objPath)
	}

	de.PrintSummary()
	return nil
}

// PrintSummary prints processing summary
func (de *DTMElevator) PrintSummary() {
	endTime := time.Now()
	duration := endTime.Sub(de.StartTime).Seconds()

	fmt.Println("\n=== DTM Elevator v1.0.0 Summary ===")
	fmt.Printf("Processing completed in %.2f seconds\n", duration)
	fmt.Printf("Files processed: %d\n", de.Stats.ProcessedFiles)
	fmt.Printf("Failed files: %d\n", len(de.Stats.FailedFiles))

	if de.Stats.ElevationStats.TotalAdjustments > 0 {
		avgAdjustment := de.Stats.ElevationStats.TotalAdjustment / float64(de.Stats.ElevationStats.TotalAdjustments)
		fmt.Println("\nElevation adjustment statistics:")
		fmt.Printf("  Total adjustments: %d\n", de.Stats.ElevationStats.TotalAdjustments)
		fmt.Printf("  Min adjustment: %.6f meters\n", de.Stats.ElevationStats.MinAdjustment)
		fmt.Printf("  Max adjustment: %.6f meters\n", de.Stats.ElevationStats.MaxAdjustment)
		fmt.Printf("  Average adjustment: %.6f meters\n", avgAdjustment)
	}

	if len(de.Stats.FailedFiles) > 0 {
		fmt.Println("\nFailed files:")
		for _, failed := range de.Stats.FailedFiles {
			fmt.Printf("- %s: %s\n", failed.Name, failed.Error)
		}
	}

	fmt.Println("===================================")
}

func main() {
	var inputDir = flag.String("input", "", "Input directory containing OBJ files (required)")
	var outputDir = flag.String("output", "", "Output directory for elevated OBJ files (required)")
	var dtmPath = flag.String("dtm", "", "Path to DTM TIF file (required)")
	var debug = flag.Bool("debug", false, "Enable debug output")
	var help = flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		fmt.Println("DTM Elevator v1.0.0")
		fmt.Println("Elevates OBJ files based on Digital Terrain Model (DTM) data")
		fmt.Println("\nUsage:")
		fmt.Printf("  %s --input <input_dir> --output <output_dir> --dtm <dtm_file.tif> [options]\n\n", os.Args[0])
		fmt.Println("Required arguments:")
		fmt.Println("  --input      Directory containing OBJ files to process")
		fmt.Println("  --output     Output directory for elevated OBJ files")
		fmt.Println("  --dtm        Path to DTM TIF file")
		fmt.Println("\nOptional arguments:")
		fmt.Println("  --debug      Enable debug output with detailed processing info")
		fmt.Println("  --help       Show this help message")
		fmt.Println("\nExample:")
		fmt.Printf("  %s --input ./buildings --output ./elevated --dtm ./terrain.tif\n", os.Args[0])
		os.Exit(0)
	}

	if *inputDir == "" || *outputDir == "" || *dtmPath == "" {
		fmt.Println("Error: --input, --output, and --dtm arguments are all required")
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

	// Validate DTM file
	if _, err := os.Stat(*dtmPath); err != nil {
		fmt.Printf("Error: Cannot access DTM file '%s': %v\n", *dtmPath, err)
		os.Exit(1)
	}

	// Convert paths to absolute
	absInputDir, err := filepath.Abs(*inputDir)
	if err != nil {
		fmt.Printf("Error: Invalid input directory '%s': %v\n", *inputDir, err)
		os.Exit(1)
	}

	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Printf("Error: Invalid output directory '%s': %v\n", *outputDir, err)
		os.Exit(1)
	}

	absDTMPath, err := filepath.Abs(*dtmPath)
	if err != nil {
		fmt.Printf("Error: Invalid DTM path '%s': %v\n", *dtmPath, err)
		os.Exit(1)
	}

	if *debug {
		fmt.Println("Debug mode enabled")
		fmt.Printf("Input Directory: %s\n", absInputDir)
		fmt.Printf("Output Directory: %s\n", absOutputDir)
		fmt.Printf("DTM File: %s\n", absDTMPath)
	}

	fmt.Println("DTM Elevator v1.0.0")
	fmt.Println("===================")

	// Create elevator instance
	elevator := NewDTMElevator(absInputDir, absOutputDir, absDTMPath, *debug)

	// Load DTM data
	if err := elevator.LoadDTM(); err != nil {
		fmt.Printf("Error loading DTM: %v\n", err)
		os.Exit(1)
	}
	defer elevator.CloseDTM()

	// Process all files
	if err := elevator.ProcessAllFiles(); err != nil {
		fmt.Printf("Error processing files: %v\n", err)
		os.Exit(1)
	}
}
