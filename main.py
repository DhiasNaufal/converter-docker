import tkinter as tk
from tkinter import ttk, filedialog, messagebox, scrolledtext
import os
import subprocess
import threading
import glob
from pathlib import Path
import sys

class ProcessingGUI:
    def __init__(self, root):
        self.root = root
        self.root.title("3D Building Processing Tool")
        self.root.geometry("800x700")
        
        # Variables
        self.data_folder = tk.StringVar()
        self.dtm_path = tk.StringVar()
        self.output_folder = tk.StringVar()
        self.batch_mode = tk.BooleanVar(value=False)
        
        self.setup_ui()
        
    def setup_ui(self):
        # Main frame
        main_frame = ttk.Frame(self.root, padding="10")
        main_frame.grid(row=0, column=0, sticky=(tk.W, tk.E, tk.N, tk.S))
        
        # Configure grid weights
        self.root.columnconfigure(0, weight=1)
        self.root.rowconfigure(0, weight=1)
        main_frame.columnconfigure(1, weight=1)
        
        # Title
        title_label = ttk.Label(main_frame, text="3D Building Processing Tool", 
                               font=("Arial", 16, "bold"))
        title_label.grid(row=0, column=0, columnspan=3, pady=(0, 20))
        
        # Data folder selection
        ttk.Label(main_frame, text="Data Folder:").grid(row=1, column=0, sticky=tk.W, pady=5)
        ttk.Entry(main_frame, textvariable=self.data_folder, width=50).grid(row=1, column=1, sticky=(tk.W, tk.E), pady=5, padx=(5, 5))
        ttk.Button(main_frame, text="Browse", command=self.browse_data_folder).grid(row=1, column=2, pady=5)
        
        # DTM file selection
        ttk.Label(main_frame, text="DTM File:").grid(row=2, column=0, sticky=tk.W, pady=5)
        ttk.Entry(main_frame, textvariable=self.dtm_path, width=50).grid(row=2, column=1, sticky=(tk.W, tk.E), pady=5, padx=(5, 5))
        ttk.Button(main_frame, text="Browse", command=self.browse_dtm_file).grid(row=2, column=2, pady=5)
        
        # Output folder selection
        ttk.Label(main_frame, text="Output Folder:").grid(row=3, column=0, sticky=tk.W, pady=5)
        ttk.Entry(main_frame, textvariable=self.output_folder, width=50).grid(row=3, column=1, sticky=(tk.W, tk.E), pady=5, padx=(5, 5))
        ttk.Button(main_frame, text="Browse", command=self.browse_output_folder).grid(row=3, column=2, pady=5)
        
        # Batch processing checkbox
        ttk.Checkbutton(main_frame, text="Batch Processing (Process all subfolders)", 
                       variable=self.batch_mode, command=self.on_batch_mode_change).grid(row=4, column=0, columnspan=3, sticky=tk.W, pady=10)
        
        # Detected folders frame
        self.folders_frame = ttk.LabelFrame(main_frame, text="Detected Folders", padding="10")
        self.folders_frame.grid(row=5, column=0, columnspan=3, sticky=(tk.W, tk.E, tk.N, tk.S), pady=10)
        self.folders_frame.columnconfigure(0, weight=1)
        
        # Listbox for detected folders
        self.folders_listbox = tk.Listbox(self.folders_frame, height=6)
        self.folders_listbox.grid(row=0, column=0, sticky=(tk.W, tk.E, tk.N, tk.S), pady=5)
        
        # Scrollbar for listbox
        folders_scrollbar = ttk.Scrollbar(self.folders_frame, orient=tk.VERTICAL, command=self.folders_listbox.yview)
        folders_scrollbar.grid(row=0, column=1, sticky=(tk.N, tk.S))
        self.folders_listbox.configure(yscrollcommand=folders_scrollbar.set)
        
        # Refresh button
        ttk.Button(self.folders_frame, text="Refresh", command=self.refresh_folders).grid(row=1, column=0, pady=5)
        
        # Progress bar
        self.progress_var = tk.StringVar(value="Ready")
        ttk.Label(main_frame, text="Status:").grid(row=6, column=0, sticky=tk.W, pady=5)
        self.status_label = ttk.Label(main_frame, textvariable=self.progress_var)
        self.status_label.grid(row=6, column=1, sticky=tk.W, pady=5)
        
        self.progress_bar = ttk.Progressbar(main_frame, mode='indeterminate')
        self.progress_bar.grid(row=7, column=0, columnspan=3, sticky=(tk.W, tk.E), pady=5)
        
        # Buttons frame
        buttons_frame = ttk.Frame(main_frame)
        buttons_frame.grid(row=8, column=0, columnspan=3, pady=20)
        
        self.start_button = ttk.Button(buttons_frame, text="Start Processing", command=self.start_processing)
        self.start_button.pack(side=tk.LEFT, padx=5)
        
        self.stop_button = ttk.Button(buttons_frame, text="Stop", command=self.stop_processing, state=tk.DISABLED)
        self.stop_button.pack(side=tk.LEFT, padx=5)
        
        # Log area
        log_frame = ttk.LabelFrame(main_frame, text="Processing Log", padding="10")
        log_frame.grid(row=9, column=0, columnspan=3, sticky=(tk.W, tk.E, tk.N, tk.S), pady=10)
        log_frame.columnconfigure(0, weight=1)
        log_frame.rowconfigure(0, weight=1)
        
        self.log_text = scrolledtext.ScrolledText(log_frame, height=10, width=80)
        self.log_text.grid(row=0, column=0, sticky=(tk.W, tk.E, tk.N, tk.S))
        
        # Configure grid weights for resizing
        main_frame.rowconfigure(5, weight=1)
        main_frame.rowconfigure(9, weight=2)
        
        # Initialize
        self.processing = False
        self.process = None
        
    def browse_data_folder(self):
        folder = filedialog.askdirectory(title="Select Data Folder")
        if folder:
            self.data_folder.set(folder)
            self.refresh_folders()
    
    def browse_dtm_file(self):
        file = filedialog.askopenfilename(
            title="Select DTM File",
            filetypes=[("TIFF files", "*.tif *.tiff"), ("All files", "*.*")]
        )
        if file:
            self.dtm_path.set(file)
    
    def browse_output_folder(self):
        folder = filedialog.askdirectory(title="Select Output Folder")
        if folder:
            self.output_folder.set(folder)
    
    def on_batch_mode_change(self):
        self.refresh_folders()
    
    def refresh_folders(self):
        self.folders_listbox.delete(0, tk.END)
        
        if not self.data_folder.get():
            return
        
        data_path = Path(self.data_folder.get())
        
        if not data_path.exists():
            return
        
        if self.batch_mode.get():
            # Batch mode: look for subfolders containing required files
            for subfolder in data_path.iterdir():
                if subfolder.is_dir():
                    if self.has_required_files(subfolder):
                        self.folders_listbox.insert(tk.END, f"✓ {subfolder.name}")
                    else:
                        self.folders_listbox.insert(tk.END, f"✗ {subfolder.name} (missing files)")
        else:
            # Single mode: check current folder
            if self.has_required_files(data_path):
                self.folders_listbox.insert(tk.END, f"✓ {data_path.name}")
            else:
                self.folders_listbox.insert(tk.END, f"✗ {data_path.name} (missing files)")
    
    def has_required_files(self, folder_path):
        """Check if folder contains required .obj, .geojson, and .txt files"""
        obj_files = list(folder_path.glob("*.obj"))
        geojson_files = list(folder_path.glob("*.geojson"))
        txt_files = list(folder_path.glob("*.txt"))
        
        return len(obj_files) > 0 and len(geojson_files) > 0 and len(txt_files) > 0
    
    def get_processing_folders(self):
        """Get list of folders to process"""
        if not self.data_folder.get():
            return []
        
        data_path = Path(self.data_folder.get())
        folders_to_process = []
        
        if self.batch_mode.get():
            # Batch mode: process all valid subfolders
            for subfolder in data_path.iterdir():
                if subfolder.is_dir() and self.has_required_files(subfolder):
                    folders_to_process.append(subfolder)
        else:
            # Single mode: process the selected folder
            if self.has_required_files(data_path):
                folders_to_process.append(data_path)
        
        return folders_to_process
    
    def validate_inputs(self):
        """Validate all required inputs"""
        if not self.data_folder.get():
            messagebox.showerror("Error", "Please select a data folder")
            return False
        
        if not self.dtm_path.get():
            messagebox.showerror("Error", "Please select a DTM file")
            return False
        
        if not self.output_folder.get():
            messagebox.showerror("Error", "Please select an output folder")
            return False
        
        if not os.path.exists(self.dtm_path.get()):
            messagebox.showerror("Error", "DTM file does not exist")
            return False
        
        folders_to_process = self.get_processing_folders()
        if not folders_to_process:
            messagebox.showerror("Error", "No valid folders found to process")
            return False
        
        return True
    
    def log_message(self, message):
        """Add message to log"""
        self.log_text.insert(tk.END, f"{message}\n")
        self.log_text.see(tk.END)
        self.root.update_idletasks()
    
    def create_shell_script(self, folder_path, subgrid_prefix):
        """Create shell script for processing a specific folder"""
        
        # Find required files
        obj_files = list(folder_path.glob("*.obj"))
        geojson_files = list(folder_path.glob("*.geojson"))
        txt_files = list(folder_path.glob("*.txt"))
        
        if not (obj_files and geojson_files and txt_files):
            raise ValueError(f"Missing required files in {folder_path}")
        
        # Use the first found file of each type
        obj_file = obj_files[0]
        geojson_file = geojson_files[0]
        txt_file = txt_files[0]
        
        # Create shell script content
        script_content = f'''#!/bin/bash

# VAR INPUT
TXT="{txt_file}"
OBJ="{obj_file}"
BO="{geojson_file}"
DTM="{self.dtm_path.get()}"

SUBGRID_PREFIX="{subgrid_prefix}"
OUTPUT_PATH="{self.output_folder.get()}"

# VAR CONSTANT : Jangan diubah kecuali untuk debugging
X=$(sed -n '1p' "$TXT" | tr -d '\\r')
Y=$(sed -n '2p' "$TXT" | tr -d '\\r')

out_separate="temp/$SUBGRID_PREFIX/separate"
out_decimate="temp/$SUBGRID_PREFIX/decimate"
out_translate="temp/$SUBGRID_PREFIX/translate"
out_semantic="temp/$SUBGRID_PREFIX/split"
out_elevate="temp/$SUBGRID_PREFIX/elevate"
out_citygml="temp/$SUBGRID_PREFIX/citygml"

final_output_citygml="$OUTPUT_PATH/$SUBGRID_PREFIX/$SUBGRID_PREFIX.gml"
final_output_3dtiles="$OUTPUT_PATH/$SUBGRID_PREFIX/$SUBGRID_PREFIX"

# Create necessary directories
mkdir -p "$out_separate"
mkdir -p "$out_decimate"
mkdir -p "$out_translate"
mkdir -p "$out_semantic"
mkdir -p "$out_elevate"
mkdir -p "$out_citygml"
mkdir -p "$OUTPUT_PATH/$SUBGRID_PREFIX"

echo "Processing $SUBGRID_PREFIX..."
echo "X coordinate: $X"
echo "Y coordinate: $Y"

# proses separation
echo "Step 1: Separation..."
go run func/separator/objseparator.go\\
    -cx=$X\\
    -cy=$Y\\
    "$OBJ"\\
    "$BO"\\
    "$out_separate"\\

# proses decimate
echo "Step 2: Decimation..."
python func/decimate/decimate.py\\
    -i "$out_separate"\\
    -o "$out_decimate"\\
    -a 25

# proses translate
echo "Step 3: Translation..."
go run func/translate/translate.go\\
    -input="$out_decimate"\\
    -output="$out_translate"\\
    -tx=$X\\
    -ty=$Y\\
    -tz=0\\

# proses elevate Z
echo "Step 4: Elevation..."
go run func/elevate/elevate.go\\
    --input "$out_translate"\\
    --output "$out_elevate"\\
    --dtm "$DTM"\\

# proses generate semantic
echo "Step 5: Semantic mapping..."
go run func/semantic/semantic-mapping.go\\
    --obj-dir "$out_elevate"\\
    --geojson "$BO"\\
    --output "$out_semantic"\\

# proses convert to CityGML Building LoD 2
echo "Step 6: Convert to CityGML..."
go run func/building-lod2/to-citygml-lod2.go\\
    -input "$out_semantic"\\
    -output "$out_citygml"\\

# proses merge CityGML Building LoD 2
echo "Step 7: Merge CityGML..."
go run func/merge-citygml/merge-building-lod2.go\\
    --input "$out_citygml"\\
    --output "$final_output_citygml"

echo "Processing completed for $SUBGRID_PREFIX"

# bersihkan temporary file
# rm -r temp/
'''
        
        return script_content
    
    def run_processing(self):
        """Run the processing in a separate thread"""
        try:
            folders_to_process = self.get_processing_folders()
            total_folders = len(folders_to_process)
            
            self.log_message(f"Starting processing of {total_folders} folder(s)...")
            
            for i, folder_path in enumerate(folders_to_process, 1):
                if not self.processing:
                    break
                
                subgrid_prefix = folder_path.name
                self.progress_var.set(f"Processing {subgrid_prefix} ({i}/{total_folders})")
                
                self.log_message(f"\n--- Processing folder {i}/{total_folders}: {subgrid_prefix} ---")
                
                try:
                    # Create shell script
                    script_content = self.create_shell_script(folder_path, subgrid_prefix)
                    
                    # Write script to temporary file
                    script_path = f"temp_script_{subgrid_prefix}.sh"
                    with open(script_path, 'w') as f:
                        f.write(script_content)
                    
                    # Make script executable (Unix/Linux/macOS)
                    if os.name != 'nt':  # Not Windows
                        os.chmod(script_path, 0o755)
                    
                    # Run the script
                    if os.name == 'nt':  # Windows
                        # Use Git Bash or WSL if available, otherwise skip
                        cmd = ['bash', script_path]
                    else:  # Unix/Linux/macOS
                        cmd = ['bash', script_path]
                    
                    self.process = subprocess.Popen(
                        cmd,
                        stdout=subprocess.PIPE,
                        stderr=subprocess.STDOUT,
                        universal_newlines=True,
                        bufsize=1
                    )
                    
                    # Read output in real-time
                    for line in self.process.stdout:
                        if not self.processing:
                            break
                        self.log_message(line.strip())
                    
                    self.process.wait()
                    
                    if self.process.returncode == 0:
                        self.log_message(f"✓ Successfully processed {subgrid_prefix}")
                    else:
                        self.log_message(f"✗ Error processing {subgrid_prefix} (exit code: {self.process.returncode})")
                    
                    # Clean up script file
                    try:
                        os.remove(script_path)
                    except:
                        pass
                        
                except Exception as e:
                    self.log_message(f"✗ Error processing {subgrid_prefix}: {str(e)}")
            
            if self.processing:
                self.log_message(f"\n=== Processing completed! ===")
                self.progress_var.set("Completed")
            else:
                self.log_message(f"\n=== Processing stopped by user ===")
                self.progress_var.set("Stopped")
                
        except Exception as e:
            self.log_message(f"Error: {str(e)}")
            self.progress_var.set("Error")
        finally:
            self.processing = False
            self.progress_bar.stop()
            self.start_button.config(state=tk.NORMAL)
            self.stop_button.config(state=tk.DISABLED)
    
    def start_processing(self):
        """Start the processing"""
        if not self.validate_inputs():
            return
        
        self.processing = True
        self.start_button.config(state=tk.DISABLED)
        self.stop_button.config(state=tk.NORMAL)
        self.progress_bar.start()
        self.log_text.delete(1.0, tk.END)
        
        # Start processing in separate thread
        thread = threading.Thread(target=self.run_processing)
        thread.daemon = True
        thread.start()
    
    def stop_processing(self):
        """Stop the processing"""
        self.processing = False
        if self.process:
            try:
                self.process.terminate()
            except:
                pass
        self.progress_var.set("Stopping...")

def main():
    root = tk.Tk()
    app = ProcessingGUI(root)
    root.mainloop()

if __name__ == "__main__":
    main()
