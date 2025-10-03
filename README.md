# 3D Building Processing Tool GUI

A user-friendly desktop application designed to orchestrate a complex pipeline for processing 3D building data. This tool provides a graphical user interface (GUI) to manage inputs, execute a series of Go and Python scripts, and monitor the progress of converting raw 3D models into a structured CityGML format.

## ✨ Features

  * **Graphical User Interface**: An intuitive interface built with Tkinter for easy selection of input/output files and folders.
  * **Batch Processing**: Process a single data folder or enable **Batch Mode** to automatically find and process all valid subfolders.
  * **Input Validation**: The tool automatically scans selected folders to ensure all required files (`.obj`, `.geojson`, `.txt`) are present before starting.
  * **Real-time Logging**: A dedicated log panel streams the real-time output from the underlying command-line scripts, providing detailed insight into the current process.
  * **Asynchronous Processing**: The processing task runs in a separate thread, ensuring the GUI remains responsive and doesn't freeze during long operations.
  * **Cancellable Operations**: A "Stop" button allows the user to safely terminate the ongoing processing task at any time.

-----

## ⚙️ The Processing Pipeline

This GUI acts as a wrapper for a multi-step command-line workflow. When you click "Start," the tool dynamically generates and executes a shell script that performs the following sequence of operations for each data folder:

1.  **Separation**: Separates individual building objects from a large `.obj` file based on a `.geojson` footprint file.
2.  **Decimation**: Reduces the polygon count of each separated 3D model to optimize its complexity.
3.  **Translation**: Translates the decimated models from a local origin to their real-world coordinates using values from the input `.txt` file.
4.  **Elevation**: Adjusts the Z-height (elevation) of the models by draping them onto a provided Digital Terrain Model (DTM).
5.  **Semantic Mapping**: Assigns semantic surfaces (e.g., Roof, Wall) to the model's polygons based on the building footprint.
6.  **CityGML Conversion**: Converts the processed and semantically enriched models into the CityGML LoD2 format.
7.  **Merge CityGML**: Merges all individual building CityGML files into a single, final `.gml` file for the processed area.

-----

## 📦 Prerequisites

Before running this tool, ensure you have the following installed and configured:

  * **Python 3.x**: Required to run the main application (`main.py`).
  * **Go (Golang)**: Required to compile and run the various `.go` scripts in the processing pipeline.
  * **Project Structure**: The tool expects a specific directory structure for its helper scripts. Your project must contain the `func/` directory with all its subdirectories (`separator`, `decimate`, etc.) as referenced in the script.

<!-- end list -->

```
your_project_root/
├── main.py
├── func/
│   ├── separator/
│   │   └── objseparator.go
│   ├── decimate/
│   │   └── decimate.py
│   ├── translate/
│   │   └── translate.go
│   ├── elevate/
│   │   └── elevate.go
│   ├── semantic/
│   │   └── semantic-mapping.go
│   ├── building-lod2/
│   │   └── to-citygml-lod2.go
│   └── merge-citygml/
│       └── merge-building-lod2.go
└── ... (other files)
```

-----

## 📁 Input Data Structure

The tool requires a specific set of input files organized in a particular way.

### 1\. Data Folder

This is the main folder containing the 3D data.

  * **Single Mode**: The Data Folder itself must contain the three required files.
  * **Batch Mode**: The Data Folder should contain multiple subfolders, where each subfolder represents a processing unit (e.g., a map tile) and contains the three required files.

**Example Structure for Batch Mode:**

```
data_folder/
├── tile_001/
│   ├── model.obj
│   ├── footprint.geojson
│   └── origin.txt
├── tile_002/
│   ├── data.obj
│   ├── boundaries.geojson
│   └── coordinates.txt
└── ...
```

#### Required Files (per folder):

  * **`*.obj`**: A Wavefront OBJ file containing the 3D mesh data for the buildings.
  * **`*.geojson`**: A GeoJSON file containing the 2D building footprints corresponding to the models in the `.obj` file.
  * **`*.txt`**: A plain text file containing the real-world coordinates of the local origin.
      * **Line 1**: The X coordinate (Easting).
      * **Line 2**: The Y coordinate (Northing).

### 2\. DTM File

  * A Digital Terrain Model in GeoTIFF format (`.tif` or `.tiff`). This is used to accurately set the elevation of the final building models.

### 3\. Output Folder

  * An empty or existing folder where the final processed files will be saved.

-----

## 🚀 How to Use

1.  **Launch the application** by running the Python script:
    ```bash
    python main.py
    ```
2.  **Select the Data Folder**: Click "Browse" to choose the main folder containing your 3D data subfolders.
3.  **Select the DTM File**: Click "Browse" to choose the `.tif` file for the terrain model.
4.  **Select the Output Folder**: Click "Browse" to choose where the results should be saved.
5.  **Choose Processing Mode**:
      * Uncheck "Batch Processing" to process only the selected Data Folder.
      * Check **"Batch Processing"** to process all valid subfolders within the Data Folder.
6.  **Refresh Folders**: Click the "Refresh" button. The "Detected Folders" list will update, showing which folders are ready for processing (✓) and which are missing required files (✗).
7.  **Start Processing**: Click the "Start Processing" button.
8.  **Monitor Progress**: Watch the status bar and the "Processing Log" for real-time updates.
9.  **Stop (Optional)**: If you need to cancel the operation, click the "Stop" button.

-----

## 📄 Output

The tool will create a subfolder within your specified **Output Folder** for each successfully processed data folder.

**Example Output Structure:**

```
output_folder/
└── tile_001/
    ├── tile_001.gml      <-- The final, merged CityGML file
    └── tile_001          <-- (This folder might be created for other outputs like 3D Tiles if the script is extended)
└── tile_002/
    ├── tile_002.gml
    └── tile_002
```

The primary output is the **`.gml`** file, which contains all the processed buildings in the CityGML LoD2 standard. The tool also creates a `temp/` directory in the project's root for intermediate files, which you may need to delete manually after processing.