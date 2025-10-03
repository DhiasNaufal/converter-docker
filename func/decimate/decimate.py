#!/usr/bin/env python3
"""
OBJ Decimator using Blender's Decimate Planar Modifier
Processes all OBJ files in a directory using Blender's built-in decimation tools.
"""

import bpy
import bmesh
import os
import sys
import argparse
import glob
from mathutils import Vector

def clear_scene():
    """Clear all objects from the scene"""
    bpy.ops.object.select_all(action='SELECT')
    bpy.ops.object.delete(use_global=False)

def import_obj(filepath):
    """Import OBJ file and return the imported object"""
    clear_scene()
    
    # Import OBJ
    bpy.ops.import_scene.obj(filepath=filepath)
    
    # Get the imported object (should be the active object)
    obj = bpy.context.active_object
    if obj is None:
        # If no active object, get the first mesh object
        for o in bpy.context.scene.objects:
            if o.type == 'MESH':
                obj = o
                break
    
    return obj

def decimate_planar(obj, angle_degrees=5.0):
    """Apply planar decimation to the object"""
    if obj is None or obj.type != 'MESH':
        print(f"Warning: Object is not a mesh, skipping decimation")
        return obj
    
    # Make sure the object is selected and active
    bpy.context.view_layer.objects.active = obj
    obj.select_set(True)
    
    # Get original face count
    original_faces = len(obj.data.polygons)
    
    # Add decimate modifier
    decimate_modifier = obj.modifiers.new(name="Decimate", type='DECIMATE')
    decimate_modifier.decimate_type = 'DISSOLVE'  # This is the planar method
    decimate_modifier.angle_limit = angle_degrees * (3.14159 / 180.0)  # Convert to radians
    decimate_modifier.use_dissolve_boundaries = False
    
    # Apply the modifier
    bpy.context.view_layer.objects.active = obj
    bpy.ops.object.modifier_apply(modifier=decimate_modifier.name)
    
    # Get final face count
    final_faces = len(obj.data.polygons)
    
    print(f"    Faces: {original_faces} → {final_faces} ({((original_faces - final_faces) / original_faces * 100):.1f}% reduction)")
    
    return obj

def export_obj(obj, filepath):
    """Export object as OBJ file"""
    if obj is None:
        print(f"Warning: No object to export")
        return False
    
    # Select only the object to export
    bpy.ops.object.select_all(action='DESELECT')
    obj.select_set(True)
    bpy.context.view_layer.objects.active = obj
    
    # Export OBJ
    bpy.ops.export_scene.obj(
        filepath=filepath,
        use_selection=True,
        use_mesh_modifiers=True,
        use_smooth_groups=True,
        use_materials=True,
        keep_vertex_order=True
    )
    
    return True

def process_obj_file(input_path, output_path, angle_degrees):
    """Process a single OBJ file"""
    print(f"Processing: {os.path.basename(input_path)}")
    
    try:
        # Import OBJ
        obj = import_obj(input_path)
        if obj is None:
            print(f"    Error: Failed to import {input_path}")
            return False
        
        print(f"    Original vertices: {len(obj.data.vertices)}")
        
        # Apply decimation
        decimated_obj = decimate_planar(obj, angle_degrees)
        
        # Create output directory if it doesn't exist
        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        
        # Export decimated OBJ
        success = export_obj(decimated_obj, output_path)
        
        if success:
            print(f"    Saved to: {output_path}")
            return True
        else:
            print(f"    Error: Failed to export {output_path}")
            return False
            
    except Exception as e:
        print(f"    Error processing {input_path}: {str(e)}")
        return False

def find_obj_files(input_dir):
    """Find all OBJ files in the input directory"""
    obj_files = []
    
    # Search for OBJ files recursively
    for root, dirs, files in os.walk(input_dir):
        for file in files:
            if file.lower().endswith('.obj'):
                obj_files.append(os.path.join(root, file))
    
    return sorted(obj_files)

def main():
    # Parse command line arguments
    parser = argparse.ArgumentParser(
        description='Decimate OBJ files using Blender\'s planar decimation method'
    )
    parser.add_argument(
        '-i', '--input',
        required=True,
        help='Input directory containing OBJ files'
    )
    parser.add_argument(
        '-o', '--output',
        required=True,
        help='Output directory for decimated OBJ files'
    )
    parser.add_argument(
        '-a', '--angle',
        type=float,
        default=5.0,
        help='Planar decimation angle threshold in degrees (default: 5.0)'
    )
    
    # Parse arguments (skip Blender's arguments)
    if '--' in sys.argv:
        args = parser.parse_args(sys.argv[sys.argv.index('--') + 1:])
    else:
        args = parser.parse_args()
    
    input_dir = os.path.abspath(args.input)
    output_dir = os.path.abspath(args.output)
    angle_degrees = args.angle
    
    # Validate input directory
    if not os.path.exists(input_dir):
        print(f"Error: Input directory '{input_dir}' does not exist")
        sys.exit(1)
    
    # Create output directory
    os.makedirs(output_dir, exist_ok=True)
    
    # Find all OBJ files
    obj_files = find_obj_files(input_dir)
    
    if not obj_files:
        print(f"No OBJ files found in '{input_dir}'")
        sys.exit(1)
    
    print(f"Found {len(obj_files)} OBJ files to process")
    print(f"Input directory: {input_dir}")
    print(f"Output directory: {output_dir}")
    print(f"Planar angle threshold: {angle_degrees}°")
    print("-" * 50)
    
    # Process each OBJ file
    success_count = 0
    total_files = len(obj_files)
    
    for i, obj_file in enumerate(obj_files, 1):
        print(f"[{i}/{total_files}] ", end="")
        
        # Calculate relative path for output
        rel_path = os.path.relpath(obj_file, input_dir)
        output_path = os.path.join(output_dir, rel_path)
        
        # Process the file
        if process_obj_file(obj_file, output_path, angle_degrees):
            success_count += 1
        
        print()  # Empty line for readability
    
    print("-" * 50)
    print(f"Processing complete!")
    print(f"Successfully processed: {success_count}/{total_files} files")
    
    if success_count < total_files:
        print(f"Failed to process: {total_files - success_count} files")

if __name__ == "__main__":
    main()
