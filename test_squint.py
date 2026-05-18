import os
import requests
import json
import time

def run_squint_test_suite():
    SQUINT_API_URL = "http://localhost:8080/api/v1/ocr"
    TEST_DIR = "test_images"
    
    # Auto-initialize local image directory if it doesn't exist
    if not os.path.exists(TEST_DIR):
        os.makedirs(TEST_DIR)
        print("==================================================")
        print(f"📁 Created folder: './{TEST_DIR}'")
        print("👉 Please drop some test images (.png, .jpg) in there and rerun!")
        print("==================================================")
        return

    # Filter out local directory contents for common image types
    valid_extensions = ('.png', '.jpg', '.jpeg', '.bmp', '.tiff')
    test_files = [f for f in os.listdir(TEST_DIR) if f.lower().endswith(valid_extensions)]

    if not test_files:
        print("==================================================")
        print(f"❌ No test images found in the './{TEST_DIR}' directory.")
        print("👉 Please add at least one image file to test the OCR engine.")
        print("==================================================")
        return

    print("==================================================")
    print("        SQUINT MICROSERVICE LOCAL TEST SUITE       ")
    print("==================================================\n")
    
    for i, file_name in enumerate(test_files, 1):
        file_path = os.path.join(TEST_DIR, file_name)
        print(f"[{i}/{len(test_files)}] Testing Local File: {file_name}")
        
        try:
            with open(file_path, 'rb') as img_file:
                # Transmit file via multipart payload containing raw image bytes
                files = {'image': (file_name, img_file, 'image/jpeg')}
                response = requests.post(SQUINT_API_URL, files=files, timeout=15)
            
            try:
                resp_json = response.json()
                # Print only the direct text engine error field if present
                if "Error" in resp_json:
                    print(f"❌ Error: {resp_json['Error']}")
                else:
                    print(f"✅ Success! Extracted Text:\n{resp_json.get('text', '')}")
            except json.JSONDecodeError:
                print(f"❌ Server crashed or failed to return JSON. Status code: {response.status_code}")
                
        except Exception as e:
            print(f"❌ Connection/Script Error: {e}")
            
        print("-" * 50 + "\n")

if __name__ == "__main__":
    run_squint_test_suite()