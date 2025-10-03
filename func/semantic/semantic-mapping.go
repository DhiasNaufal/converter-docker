package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const Version = "2.0.0"

// Color represents RGBA color values
type Color struct {
	R, G, B, A float64
}

// Colors definition with alpha channel
var Colors = map[string]Color{
	"Roof":   {0.6627, 0.2627, 0.0863, 1.0}, // Orange
	"Wall":   {0.8314, 0.8314, 0.8471, 1.0}, // Grey
	"Ground": {0.82, 0.41, 0.12, 1.0},       // Chocolate
}

// Vector3 represents a 3D vector
type Vector3 struct {
	X, Y, Z float64
}

// Face represents a mesh face with vertex indices
type Face []int

// Polygon represents a 2D polygon
type Polygon struct {
	Coordinates [][]float64
}

// GeoJSONFeature represents a GeoJSON feature
type GeoJSONFeature struct {
	Geometry struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

// GeoJSON represents the GeoJSON structure
type GeoJSON struct {
	Features []GeoJSONFeature `json:"features"`
}

// OptimizedFaceGroup represents faces grouped by material with optimized vertices
type OptimizedFaceGroup struct {
	Material          string
	Faces             []Face
	OptimizedVertices []Vector3
	VertexMapping     map[int]int // old index -> new index
}

// MeshAnalyzer handles mesh analysis and validation
type MeshAnalyzer struct{}

// NewMeshAnalyzer creates a new MeshAnalyzer
func NewMeshAnalyzer() *MeshAnalyzer {
	return &MeshAnalyzer{}
}

// AnalyzeZDistribution analyzes Z-coordinate distribution to find ground level
func (ma *MeshAnalyzer) AnalyzeZDistribution(zValues []float64) float64 {
	if len(zValues) == 0 {
		return 0.0
	}

	// Create histogram of Z values
	minZ := zValues[0]
	maxZ := zValues[0]
	for _, z := range zValues {
		if z < minZ {
			minZ = z
		}
		if z > maxZ {
			maxZ = z
		}
	}

	bins := 50
	binWidth := (maxZ - minZ) / float64(bins)
	if binWidth == 0 {
		return minZ
	}

	hist := make([]int, bins)
	for _, z := range zValues {
		binIndex := int((z - minZ) / binWidth)
		if binIndex >= bins {
			binIndex = bins - 1
		}
		hist[binIndex]++
	}

	// Find the lowest significant peak
	maxCount := 0
	for _, count := range hist {
		if count > maxCount {
			maxCount = count
		}
	}

	significantThreshold := float64(maxCount) * 0.1
	for i, count := range hist {
		if float64(count) > significantThreshold {
			return minZ + float64(i)*binWidth
		}
	}

	return minZ
}

// GetFaceCentroid calculates the centroid of a face
func (ma *MeshAnalyzer) GetFaceCentroid(vertices []Vector3, face Face) Vector3 {
	var sum Vector3
	for _, idx := range face {
		sum.X += vertices[idx].X
		sum.Y += vertices[idx].Y
		sum.Z += vertices[idx].Z
	}
	count := float64(len(face))
	return Vector3{sum.X / count, sum.Y / count, sum.Z / count}
}

// GeometryValidator handles geometric validation and consistency checks
type GeometryValidator struct {
	Tolerance float64
}

// NewGeometryValidator creates a new GeometryValidator
func NewGeometryValidator(tolerance float64) *GeometryValidator {
	return &GeometryValidator{Tolerance: tolerance}
}

// ValidateGroundClassification validates if a face should be classified as ground
func (gv *GeometryValidator) ValidateGroundClassification(vertices []Vector3, face Face, groundHeight float64) bool {
	var avgZ float64
	for _, idx := range face {
		avgZ += vertices[idx].Z
	}
	avgZ /= float64(len(face))

	// Check if face is at ground level
	if math.Abs(avgZ-groundHeight) > gv.Tolerance {
		return false
	}

	// Check if face is horizontal
	normal := gv.GetFaceNormal(vertices, face)
	return math.Abs(normal.Z) > 0.95
}

// GetFaceNormal calculates normalized face normal
func (gv *GeometryValidator) GetFaceNormal(vertices []Vector3, face Face) Vector3 {
	if len(face) < 3 {
		return Vector3{0, 0, 1}
	}

	v0 := vertices[face[0]]
	v1 := vertices[face[1]]
	v2 := vertices[face[2]]

	edge1 := Vector3{v1.X - v0.X, v1.Y - v0.Y, v1.Z - v0.Z}
	edge2 := Vector3{v2.X - v0.X, v2.Y - v0.Y, v2.Z - v0.Z}

	normal := Vector3{
		edge1.Y*edge2.Z - edge1.Z*edge2.Y,
		edge1.Z*edge2.X - edge1.X*edge2.Z,
		edge1.X*edge2.Y - edge1.Y*edge2.X,
	}

	magnitude := math.Sqrt(normal.X*normal.X + normal.Y*normal.Y + normal.Z*normal.Z)
	if magnitude == 0 {
		return Vector3{0, 0, 1}
	}

	return Vector3{normal.X / magnitude, normal.Y / magnitude, normal.Z / magnitude}
}

// Statistics holds processing statistics
type Statistics struct {
	ProcessedFiles        int
	FailedFiles           []FailedFile
	ClassificationChanges int
	SplitFiles            map[string]int         // Track split files per material
	VertexOptimization    map[string]VertexStats // Track vertex optimization per material
}

// VertexStats tracks vertex optimization statistics
type VertexStats struct {
	OriginalVertices  int
	OptimizedVertices int
	ReductionPercent  float64
}

// FailedFile represents a failed file with error message
type FailedFile struct {
	Name  string
	Error string
}

// BuildingColorizer main class
type BuildingColorizer struct {
	ObjDir              string
	OutputDir           string
	GeoJSONPath         string
	BuildingOutlines    map[string]Polygon
	MeshAnalyzer        *MeshAnalyzer
	GeometryValidator   *GeometryValidator
	ClassificationCache map[int]string
	Stats               Statistics
	StartTime           time.Time
	Debug               bool
}

// NewBuildingColorizer creates a new BuildingColorizer
func NewBuildingColorizer(objDir, outputDir, geoJSONPath string, debug bool) *BuildingColorizer {
	bc := &BuildingColorizer{
		ObjDir:              objDir,
		OutputDir:           outputDir,
		GeoJSONPath:         geoJSONPath,
		MeshAnalyzer:        NewMeshAnalyzer(),
		GeometryValidator:   NewGeometryValidator(0.01),
		ClassificationCache: make(map[int]string),
		StartTime:           time.Now(),
		Debug:               debug,
		Stats: Statistics{
			SplitFiles:         make(map[string]int),
			VertexOptimization: make(map[string]VertexStats),
		},
	}

	bc.BuildingOutlines = bc.loadAllBuildingOutlines()
	return bc
}

// LoadObjFile loads vertices and faces from OBJ file
func (bc *BuildingColorizer) LoadObjFile(objPath string) ([]Vector3, []Face, error) {
	file, err := os.Open(objPath)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	var vertices []Vector3
	var faces []Face

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) == 0 {
			continue
		}

		switch parts[0] {
		case "v":
			if len(parts) >= 4 {
				x, err1 := strconv.ParseFloat(parts[1], 64)
				y, err2 := strconv.ParseFloat(parts[2], 64)
				z, err3 := strconv.ParseFloat(parts[3], 64)
				if err1 == nil && err2 == nil && err3 == nil {
					vertices = append(vertices, Vector3{x, y, z})
				} else {
					if bc.Debug {
						fmt.Printf("Warning: Invalid vertex at line %d in %s: %s\n", lineNum, filepath.Base(objPath), line)
					}
				}
			}
		case "f":
			if len(parts) >= 4 {
				var face Face
				validFace := true
				for i := 1; i < len(parts); i++ {
					// Handle different face formats (v, v/vt, v/vt/vn)
					vertexStr := strings.Split(parts[i], "/")[0]
					if vertexIdx, err := strconv.Atoi(vertexStr); err == nil {
						idx := vertexIdx - 1 // OBJ indices start at 1
						if idx >= 0 && idx < len(vertices) {
							face = append(face, idx)
						} else {
							validFace = false
							if bc.Debug {
								fmt.Printf("Warning: Invalid vertex index %d at line %d in %s\n", vertexIdx, lineNum, filepath.Base(objPath))
							}
							break
						}
					} else {
						validFace = false
						break
					}
				}
				if validFace && len(face) >= 3 {
					faces = append(faces, face)
				}
			}
		}
	}

	if len(vertices) == 0 || len(faces) == 0 {
		return nil, nil, fmt.Errorf("no valid vertices or faces found")
	}

	return vertices, faces, nil
}

// loadAllBuildingOutlines loads building outlines from GeoJSON
func (bc *BuildingColorizer) loadAllBuildingOutlines() map[string]Polygon {
	buildingOutlines := make(map[string]Polygon)

	data, err := ioutil.ReadFile(bc.GeoJSONPath)
	if err != nil {
		fmt.Printf("Error loading GeoJSON: %v\n", err)
		return buildingOutlines
	}

	var geoJSON GeoJSON
	if err := json.Unmarshal(data, &geoJSON); err != nil {
		fmt.Printf("Error parsing GeoJSON: %v\n", err)
		return buildingOutlines
	}

	for _, feature := range geoJSON.Features {
		if feature.Geometry.Type == "Polygon" || feature.Geometry.Type == "MultiPolygon" {
			// Simplified polygon handling
			key := fmt.Sprintf("polygon_%d", len(buildingOutlines))
			buildingOutlines[key] = Polygon{}
		}
	}

	fmt.Printf("Loaded %d valid building outlines\n", len(buildingOutlines))
	return buildingOutlines
}

// ProcessMesh processes mesh data and creates optimized face groups
func (bc *BuildingColorizer) ProcessMesh(vertices []Vector3, faces []Face) (map[string]*OptimizedFaceGroup, float64) {
	// Find ground level using distribution analysis
	zValues := make([]float64, len(vertices))
	for i, v := range vertices {
		zValues[i] = v.Z
	}
	groundHeight := bc.MeshAnalyzer.AnalyzeZDistribution(zValues)

	// Initialize face groups with vertex tracking
	faceGroups := make(map[string]*OptimizedFaceGroup)
	for material := range Colors {
		faceGroups[material] = &OptimizedFaceGroup{
			Material:      material,
			Faces:         []Face{},
			VertexMapping: make(map[int]int),
		}
	}

	// Track which vertices are used by each material
	usedVertices := make(map[string]map[int]bool)
	for material := range Colors {
		usedVertices[material] = make(map[int]bool)
	}

	// Process each face and group by material
	for _, face := range faces {
		material := bc.classifyFaceWithContext(vertices, face, groundHeight, []int{})

		if group, exists := faceGroups[material]; exists {
			group.Faces = append(group.Faces, face)
			// Track which vertices are used by this material
			for _, vertexIdx := range face {
				usedVertices[material][vertexIdx] = true
			}
		}
	}

	// Optimize vertices for each material group
	for material, group := range faceGroups {
		bc.optimizeVerticesForGroup(vertices, group, usedVertices[material])

		// Record optimization statistics
		originalCount := len(vertices)
		optimizedCount := len(group.OptimizedVertices)
		reductionPercent := 0.0
		if originalCount > 0 {
			reductionPercent = float64(originalCount-optimizedCount) / float64(originalCount) * 100
		}

		bc.Stats.VertexOptimization[material] = VertexStats{
			OriginalVertices:  originalCount,
			OptimizedVertices: optimizedCount,
			ReductionPercent:  reductionPercent,
		}
	}

	return faceGroups, groundHeight
}

// optimizeVerticesForGroup creates optimized vertex list and mapping for a material group
func (bc *BuildingColorizer) optimizeVerticesForGroup(allVertices []Vector3, group *OptimizedFaceGroup, usedVertexIndices map[int]bool) {
	if len(usedVertexIndices) == 0 {
		return
	}

	// Create sorted list of used vertex indices for consistent ordering
	var sortedIndices []int
	for idx := range usedVertexIndices {
		sortedIndices = append(sortedIndices, idx)
	}
	sort.Ints(sortedIndices)

	// Create optimized vertex list and mapping
	group.OptimizedVertices = make([]Vector3, len(sortedIndices))
	newIndex := 0

	for _, oldIndex := range sortedIndices {
		group.OptimizedVertices[newIndex] = allVertices[oldIndex]
		group.VertexMapping[oldIndex] = newIndex
		newIndex++
	}

	if bc.Debug {
		fmt.Printf("    %s: Optimized from %d to %d vertices (%.1f%% reduction)\n",
			group.Material, len(allVertices), len(group.OptimizedVertices),
			float64(len(allVertices)-len(group.OptimizedVertices))/float64(len(allVertices))*100)
	}
}

// classifyFaceWithContext classifies face considering neighboring geometry
func (bc *BuildingColorizer) classifyFaceWithContext(vertices []Vector3, face Face, groundHeight float64, neighbors []int) string {
	// Get face properties
	normal := bc.GeometryValidator.GetFaceNormal(vertices, face)

	// Basic classification
	var baseClass string
	if bc.GeometryValidator.ValidateGroundClassification(vertices, face, groundHeight) {
		baseClass = "Ground"
	} else if math.Abs(normal.Z) < 0.1 { // Nearly vertical
		baseClass = "Wall"
	} else {
		baseClass = "Roof"
	}

	return baseClass
}

// CreateSeparateObjFiles creates separate optimized OBJ files for each material
func (bc *BuildingColorizer) CreateSeparateObjFiles(objPath string, faceGroups map[string]*OptimizedFaceGroup) error {
	baseName := strings.TrimSuffix(filepath.Base(objPath), ".obj")

	for material, group := range faceGroups {
		if len(group.Faces) == 0 {
			if bc.Debug {
				fmt.Printf("  Skipping %s (no faces)\n", material)
			}
			continue // Skip materials with no faces
		}

		// Create filename with material suffix
		var suffix string
		switch material {
		case "Ground":
			suffix = "-ground"
		case "Wall":
			suffix = "-wall"
		case "Roof":
			suffix = "-roof"
		}

		outputPath := filepath.Join(bc.OutputDir, baseName+suffix+".obj")
		mtlPath := baseName + suffix + ".mtl"

		// Create optimized OBJ file
		if err := bc.createOptimizedObjFile(outputPath, mtlPath, group); err != nil {
			return fmt.Errorf("failed to create %s: %v", outputPath, err)
		}

		// Create MTL file
		if err := bc.createMtlFile(filepath.Join(bc.OutputDir, mtlPath), material); err != nil {
			return fmt.Errorf("failed to create %s: %v", mtlPath, err)
		}

		bc.Stats.SplitFiles[material]++
		if bc.Debug {
			fmt.Printf("  Created %s with %d vertices and %d faces\n",
				filepath.Base(outputPath), len(group.OptimizedVertices), len(group.Faces))
		}
	}

	return nil
}

// createOptimizedObjFile creates an individual optimized OBJ file for a specific material
func (bc *BuildingColorizer) createOptimizedObjFile(objPath, mtlPath string, group *OptimizedFaceGroup) error {
	file, err := os.Create(objPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString(fmt.Sprintf("# Generated by Building Colorizer v%s - %s (Optimized)\n", Version, group.Material))
	writer.WriteString(fmt.Sprintf("# Vertices: %d, Faces: %d\n", len(group.OptimizedVertices), len(group.Faces)))
	writer.WriteString(fmt.Sprintf("mtllib %s\n", mtlPath))
	writer.WriteString("\n")

	// Write optimized vertices
	for _, vertex := range group.OptimizedVertices {
		writer.WriteString(fmt.Sprintf("v %.6f %.6f %.6f\n", vertex.X, vertex.Y, vertex.Z))
	}
	writer.WriteString("\n")

	// Write material usage and faces with remapped indices
	writer.WriteString(fmt.Sprintf("usemtl %s\n", group.Material))
	for _, face := range group.Faces {
		writer.WriteString("f")
		for _, oldIdx := range face {
			newIdx := group.VertexMapping[oldIdx] + 1 // OBJ indices start at 1
			writer.WriteString(fmt.Sprintf(" %d", newIdx))
		}
		writer.WriteString("\n")
	}

	return nil
}

// createMtlFile creates a material file for a specific material
func (bc *BuildingColorizer) createMtlFile(mtlPath, material string) error {
	file, err := os.Create(mtlPath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	color := Colors[material]

	writer.WriteString(fmt.Sprintf("# Generated by Building Colorizer v%s - %s\n\n", Version, material))
	writer.WriteString(fmt.Sprintf("newmtl %s\n", material))
	writer.WriteString("Ka 0.000 0.000 0.000\n")
	writer.WriteString(fmt.Sprintf("Kd %.6f %.6f %.6f\n", color.R, color.G, color.B))
	writer.WriteString("Ks 0.000 0.000 0.000\n")
	writer.WriteString(fmt.Sprintf("d %.6f\n", color.A))
	writer.WriteString("illum 1\n")

	return nil
}

// ProcessBuilding processes a single building and splits it into optimized separate files
func (bc *BuildingColorizer) ProcessBuilding(objPath string) {
	if bc.Debug {
		fmt.Printf("\nProcessing: %s\n", filepath.Base(objPath))
	}

	// Load mesh data
	if bc.Debug {
		fmt.Println("  Loading mesh data...")
	}
	vertices, faces, err := bc.LoadObjFile(objPath)
	if err != nil {
		fmt.Printf("  Failed to load mesh data for %s: %v\n", filepath.Base(objPath), err)
		bc.Stats.FailedFiles = append(bc.Stats.FailedFiles, FailedFile{filepath.Base(objPath), err.Error()})
		return
	}

	if bc.Debug {
		fmt.Printf("  Loaded %d vertices and %d faces\n", len(vertices), len(faces))
	}

	// Process mesh and create optimized face groups
	if bc.Debug {
		fmt.Println("  Processing mesh and optimizing vertices...")
	}
	faceGroups, groundHeight := bc.ProcessMesh(vertices, faces)
	if bc.Debug {
		fmt.Printf("  Ground height detected: %.2f\n", groundHeight)
	}

	// Print face and vertex distribution
	for material, group := range faceGroups {
		if len(group.Faces) > 0 {
			if bc.Debug {
				fmt.Printf("  %s: %d faces, %d vertices\n", material, len(group.Faces), len(group.OptimizedVertices))
			}
		}
	}

	// Create separate optimized OBJ files for each material
	if bc.Debug {
		fmt.Println("  Creating optimized OBJ files...")
	}
	if err := bc.CreateSeparateObjFiles(objPath, faceGroups); err != nil {
		bc.Stats.FailedFiles = append(bc.Stats.FailedFiles, FailedFile{filepath.Base(objPath), fmt.Sprintf("File splitting failed: %v", err)})
		return
	}

	bc.Stats.ProcessedFiles++
	if bc.Debug {
		fmt.Printf("  Successfully processed and optimized %s\n", filepath.Base(objPath))
	}
}

// ProcessAllBuildings processes all buildings in directory
func (bc *BuildingColorizer) ProcessAllBuildings() {
	// Ensure output directory exists
	if err := os.MkdirAll(bc.OutputDir, 0755); err != nil {
		log.Fatalf("Error creating output directory: %v", err)
	}

	pattern := filepath.Join(bc.ObjDir, "*.obj")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		log.Fatalf("Error finding OBJ files: %v", err)
	}

	if len(matches) == 0 {
		fmt.Printf("No OBJ files found in directory: %s\n", bc.ObjDir)
		return
	}

	fmt.Printf("Found %d OBJ files to process\n", len(matches))
	fmt.Printf("Output directory: %s\n", bc.OutputDir)

	for _, objPath := range matches {
		bc.ProcessBuilding(objPath)
	}

	bc.PrintSummary()
}

// PrintSummary prints detailed processing summary
func (bc *BuildingColorizer) PrintSummary() {
	endTime := time.Now()
	duration := endTime.Sub(bc.StartTime).Seconds()

	fmt.Println("\n=== Building Colorizer v2.0.0 Summary ===")
	fmt.Printf("Processing completed in %.2f seconds\n", duration)
	fmt.Printf("Original files processed: %d\n", bc.Stats.ProcessedFiles)
	fmt.Printf("Output directory: %s\n", bc.OutputDir)

	fmt.Println("\nSplit files created:")
	totalSplitFiles := 0
	for material, count := range bc.Stats.SplitFiles {
		fmt.Printf("  %s files: %d\n", material, count)
		totalSplitFiles += count
	}
	fmt.Printf("  Total split files: %d\n", totalSplitFiles)

	fmt.Println("\nVertex optimization results:")
	for material, stats := range bc.Stats.VertexOptimization {
		if bc.Stats.SplitFiles[material] > 0 {
			fmt.Printf("  %s: %d → %d vertices (%.1f%% reduction)\n",
				material, stats.OriginalVertices, stats.OptimizedVertices, stats.ReductionPercent)
		}
	}

	fmt.Printf("\nClassification adjustments: %d\n", bc.Stats.ClassificationChanges)
	fmt.Printf("Failed files: %d\n", len(bc.Stats.FailedFiles))

	if len(bc.Stats.FailedFiles) > 0 {
		fmt.Println("\nFailed files:")
		for _, failed := range bc.Stats.FailedFiles {
			fmt.Printf("- %s: %s\n", failed.Name, failed.Error)
		}
	}
	fmt.Println("=====================================")
}

func main() {
	var objDir = flag.String("obj-dir", "", "Directory containing OBJ files (required)")
	var outputDir = flag.String("output", "", "Output directory for split files (required)")
	var geoJSON = flag.String("geojson", "", "Path to GeoJSON building outlines (required)")
	var debug = flag.Bool("debug", false, "Enable debug output")
	var help = flag.Bool("help", false, "Show help message")
	flag.Parse()

	if *help {
		fmt.Println("Building Colorizer v2.0.0 - Optimized File Splitter")
		fmt.Println("Splits OBJ files into optimized separate files for each material type")
		fmt.Println("\nUsage:")
		fmt.Printf("  %s --obj-dir <input_dir> --output <output_dir> --geojson <geojson_file> [options]\n\n", os.Args[0])
		fmt.Println("Required arguments:")
		fmt.Println("  --obj-dir    Directory containing OBJ files to process")
		fmt.Println("  --output     Output directory for split and optimized files")
		fmt.Println("  --geojson    Path to GeoJSON file with building outlines")
		fmt.Println("\nOptional arguments:")
		fmt.Println("  --debug      Enable debug output with detailed vertex optimization info")
		fmt.Println("  --help       Show this help message")
		fmt.Println("\nExample:")
		fmt.Printf("  %s --obj-dir ./input --output ./output --geojson ./outlines.geojson\n", os.Args[0])
		fmt.Println("\nOutput:")
		fmt.Println("  For each input file 'building.obj', creates optimized files:")
		fmt.Println("    - building_ground.obj (ground faces with minimal vertices)")
		fmt.Println("    - building_wall.obj   (wall faces with minimal vertices)")
		fmt.Println("    - building_roof.obj   (roof faces with minimal vertices)")
		fmt.Println("  Each with corresponding .mtl files")
		fmt.Println("\nOptimization:")
		fmt.Println("  - Removes unused vertices from each split file")
		fmt.Println("  - Remaps face indices to use optimized vertex list")
		fmt.Println("  - Significantly reduces file sizes")
		os.Exit(0)
	}

	if *objDir == "" || *outputDir == "" || *geoJSON == "" {
		fmt.Println("Error: --obj-dir, --output, and --geojson arguments are all required")
		fmt.Println("Use --help for usage information")
		os.Exit(1)
	}

	// Validate input directory
	if info, err := os.Stat(*objDir); err != nil {
		fmt.Printf("Error: Cannot access obj-dir '%s': %v\n", *objDir, err)
		os.Exit(1)
	} else if !info.IsDir() {
		fmt.Printf("Error: obj-dir '%s' is not a directory\n", *objDir)
		os.Exit(1)
	}

	// Validate GeoJSON file
	if _, err := os.Stat(*geoJSON); err != nil {
		fmt.Printf("Error: Cannot access geojson file '%s': %v\n", *geoJSON, err)
		os.Exit(1)
	}

	// Convert output directory to absolute path
	absOutputDir, err := filepath.Abs(*outputDir)
	if err != nil {
		fmt.Printf("Error: Invalid output directory '%s': %v\n", *outputDir, err)
		os.Exit(1)
	}

	if *debug {
		fmt.Println("Debug mode enabled")
		fmt.Printf("Input Directory: %s\n", *objDir)
		fmt.Printf("Output Directory: %s\n", absOutputDir)
		fmt.Printf("GeoJSON File: %s\n", *geoJSON)
	}

	fmt.Println("Building Colorizer v2.0.0 - Optimized File Splitter")
	fmt.Println("===================================================")

	colorizer := NewBuildingColorizer(*objDir, absOutputDir, *geoJSON, *debug)
	colorizer.ProcessAllBuildings()
}
