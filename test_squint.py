import os
import requests
import json
import time

def run_squint_test_suite():
    # Note: Using the POST endpoint now instead of passing URL parameters
    SQUINT_API_URL = "http://localhost:8080/api/v1/ocr"
    
    # Create a local test directory name
    TEST_DIR = "test_images"
    
    # Check if the folder exists, if not, create it and remind the user
    if not os.path.exists(TEST_DIR):
        os.makedirs(TEST_DIR)
        print("==================================================")
        print(f"📁 Created folder: './{TEST_DIR}'")
        print("👉 Please drop some test images (.png, .jpg) in there and rerun!")
        print("==================================================")
        return

    # Find all images inside the test_images directory
    valid_extensions = ('.png', '.jpg', '.jpeg', '.bmp', '.tiff')
    test_files = [f for f in os.listdir(TEST_DIR) if f.lower().endswith(valid_extensions)]

    if not test_files:
        print("==================================================")
        print(f"❌ No test images found in the './{TEST_DIR}' directory.")
        print("👉 Please add at least one image file to test the OCR engine.")
        print("==================================================")
        return

    print("==================================================")
    print("     SQUINT MICROSERVICE LOCAL TEST SUITE         ")
    print("==================================================\n")
    
    for i, file_name in enumerate(test_files, 1):
        print(f"Testing file: {file_name}")
        file_path = os.path.join(TEST_DIR, file_name)
        print(f"[{i}/{len(test_files)}] Testing Local File: {file_name}")
        
        start_time = time.time()
        print("⏳ Sending request to Squint...")
        
        try:
            # Open the file in binary read mode ('rb')
            with open(file_path, 'rb') as img_file:
                # Prepare the multipart form data payload
                # 'image' matches the key we specified in Go: request.FormFile("image")
                files = {'image': (file_name, img_file, 'image/jpeg')}
                
                # Send the POST request to Squint
                response = requests.post(SQUINT_API_URL, files=files, timeout=15)
                
            elapsed_time = time.time() - start_time
            print(f"Server Response Code: {response.status_code} (Took {elapsed_time:.2f}s)")
            
            # Print the JSON payload returned by Squint
            try:
                print("Result Payload:")
                print(json.dumps(response.json(), indent=4))
            except json.JSONDecodeError:
                print(f"Raw Output (Non-JSON): {response.text}")
                
        except requests.exceptions.Timeout:
            print("❌ Error: The request timed out.")
        except requests.exceptions.ConnectionError:
            print("❌ Error: Could not connect to Squint. Is the container running?")
        except Exception as e:
            print(f"❌ Unexpected script error processing {file_name}: {e}")
            
        print("-" * 50 + "\n")

if __name__ == "__main__":
    run_squint_test_suite()