#!/usr/bin/env python3

import yaml
import sys
import os
from pathlib import Path

def clean_k8s_resource(doc):
    """Clean runtime-generated fields from a Kubernetes resource"""
    if doc is None or 'metadata' not in doc:
        return doc
    
    metadata = doc['metadata']
    
    # Remove runtime fields
    runtime_fields = [
        'creationTimestamp',
        'resourceVersion',
        'uid',
        'generation',
        'finalizers',
        'selfLink',
        'managedFields'
    ]
    
    for field in runtime_fields:
        metadata.pop(field, None)
    
    # Remove status section (should not be applied)
    if 'status' in doc:
        doc.pop('status')
    
    # Remove auto-generated labels and annotations
    if 'labels' in metadata:
        labels_to_remove = [
            'propagationpolicy.karmada.io/permanent-id',
            'clusterpropagationpolicy.karmada.io/permanent-id',
            'overridepolicy.karmada.io/permanent-id'
        ]
        for label in labels_to_remove:
            metadata['labels'].pop(label, None)
        # Remove empty labels dict
        if not metadata['labels']:
            metadata.pop('labels')
    
    if 'annotations' in metadata:
        annotations_to_remove = [
            'kubectl.kubernetes.io/last-applied-configuration',
            'propagationpolicy.karmada.io/permanent-id',
            'clusterpropagationpolicy.karmada.io/permanent-id',
            'clusterpropagationpolicy.karmada.io/name',
            'overridepolicy.karmada.io/permanent-id',
            'karmada.io/applied',
            'karmada.io/last-applied'
        ]
        for ann in annotations_to_remove:
            metadata['annotations'].pop(ann, None)
        # Remove empty annotations dict
        if not metadata['annotations']:
            metadata.pop('annotations')
    
    return doc

def clean_yaml_file(filepath):
    """Remove runtime-generated fields from YAML file"""
    with open(filepath, 'r') as f:
        docs = yaml.safe_load_all(f)
        cleaned_docs = []
        
        for doc in docs:
            if doc is None:
                continue
            
            # Handle List type resources (with items array)
            if doc.get('kind') == 'List' and 'items' in doc:
                # Clean the list itself
                doc = clean_k8s_resource(doc)
                # Clean each item in the list
                if 'items' in doc:
                    cleaned_items = []
                    for item in doc['items']:
                        if item is not None:
                            cleaned_item = clean_k8s_resource(item)
                            if cleaned_item:
                                cleaned_items.append(cleaned_item)
                    doc['items'] = cleaned_items
            else:
                # Clean single resource
                doc = clean_k8s_resource(doc)
            
            cleaned_docs.append(doc)
        
        # Write cleaned YAML
        with open(filepath, 'w') as f:
            if len(cleaned_docs) == 1:
                yaml.dump(cleaned_docs[0], f, default_flow_style=False, sort_keys=False)
            else:
                yaml.dump_all(cleaned_docs, f, default_flow_style=False, sort_keys=False)
    
    return filepath

def clean_directory(dirpath):
    """Clean all YAML files in directory"""
    dirpath = Path(dirpath)
    
    if not dirpath.exists():
        print(f"Directory not found: {dirpath}")
        return
    
    yaml_files = list(dirpath.glob('*.yaml')) + list(dirpath.glob('*.yml'))
    
    if not yaml_files:
        print(f"No YAML files found in: {dirpath}")
        return
    
    print(f"Cleaning {len(yaml_files)} YAML files in {dirpath}")
    
    for yaml_file in yaml_files:
        try:
            clean_yaml_file(yaml_file)
            print(f"  ✓ {yaml_file.name}")
        except Exception as e:
            print(f"  ✗ {yaml_file.name}: {e}")

def main():
    if len(sys.argv) > 1:
        # Clean specific file or directory
        target = sys.argv[1]
        if os.path.isfile(target):
            clean_yaml_file(target)
            print(f"Cleaned: {target}")
        elif os.path.isdir(target):
            clean_directory(target)
        else:
            print(f"Target not found: {target}")
            sys.exit(1)
    else:
        # Clean all configurations
        script_dir = Path(__file__).parent
        
        print("=" * 50)
        print("Cleaning Karmada Configuration Files")
        print("=" * 50)
        print()
        
        for subdir in ['propagation-policies', 'override-policies', 'queues', 'rbac']:
            clean_directory(script_dir / subdir)
        
        print()
        print("=" * 50)
        print("Configuration files cleaned successfully!")
        print("=" * 50)

if __name__ == '__main__':
    main()