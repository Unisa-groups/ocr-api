import requests
import json
import time

def run_squint_test_suite():
    SQUINT_API_URL = "http://localhost:8080/api/v1/ocr"
    
    # Define a matrix of different image scenarios to test the OCR engine boundaries
    test_cases = [
        {
            "name": "Standard Clean Text (PNG)",
            "url": "https://raw.githubusercontent.com/otiai10/gosseract/main/test/data/001-helloworld.png"
        },
        {
            "name": "Slightly Distorted Text (PNG)",
            "url": "https://raw.githubusercontent.com/otiai10/gosseract/main/test/data/002-book.png"
        },
        {
            "name": "Deliberate 404 Error (Edge Case Test)",
            "url": "https://raw.githubusercontent.com/otiai10/gosseract/main/test/data/this-file-does-not-exist.jpg"
        }
    ]
    
    print("==================================================")
    print("        SQUINT MICROSERVICE TEST SUITE            ")
    print("==================================================\n")
    
    for i, case in enumerate(test_cases, 1):
        print(f"[{i}/{len(test_cases)}] Testing: {case['name']}")
        print(f"Target URL: {case['url']}")
        
        payload = {"image_url": case["url"]}
        start_time = time.time()
        
        try:
            # Fire request to the Go microservice
            response = requests.get(SQUINT_API_URL, params=payload, timeout=15)
            elapsed_time = time.time() - start_time
            
            print(f"Server Response Code: {response.status_code} (Took {elapsed_time:.2f}s)")
            
            # Pretty-print the structured JSON payload returned by Squint
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
            print(f"❌ Unexpected script error: {e}")
            
        print("-" * 50 + "\n")

if __name__ == "__main__":
    run_squint_test_suite()