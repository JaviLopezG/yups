import os
import time
import logging
import json
import requests
from flask import Flask, request, jsonify, abort
from collections import deque, defaultdict
from datetime import datetime

# --- CONFIGURATION ---
app = Flask(__name__)

# Configurable via environment variables
HF_TOKEN = os.environ.get("HF_TOKEN")
# Model ID: We use the standard HF Inference URL
HF_MODEL_ID = os.environ.get("HF_MODEL_ID", "google/gemma-2-27b-it") 
HF_API_URL = f"https://api-inference.huggingface.co/models/{HF_MODEL_ID}"

# Security
YUPS_CLIENT_HEADER = os.environ.get("YUPS_CLIENT_HEADER", "X-Yups-Client-Version")
YUPS_CLIENT_SECRET = os.environ.get("YUPS_CLIENT_SECRET", "yups-v1-secret-key")

# Logging Setup
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s | %(levelname)s | %(message)s',
    handlers=[
        logging.FileHandler("server_requests.log"),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger("YupsAPI")

# --- IN-MEMORY STATE (Not persistent across restarts) ---
# Rate Limiting: {ip: [timestamp1, timestamp2, ...]}
ip_access_history = defaultdict(list)

# Stats: Rolling window of last 10 latency measurements
latency_history = deque(maxlen=10)

def clean_old_timestamps(ip):
    """Removes timestamps older than 1 hour to save memory."""
    now = time.time()
    # Keep only timestamps within the last hour (3600 seconds)
    ip_access_history[ip] = [t for t in ip_access_history[ip] if now - t < 3600]

def check_rate_limit(ip):
    """
    Enforces:
    - Max 100 requests per hour (Reject)
    - Max 5 requests per minute (Pause 15s)
    """
    clean_old_timestamps(ip)
    now = time.time()
    history = ip_access_history[ip]
    
    # 1. Hourly Limit Check
    if len(history) >= 100:
        logger.warning(f"⛔ IP {ip} blocked (Hourly limit exceeded).")
        abort(429, description="Hourly rate limit exceeded (100 req/h).")

    # 2. Minute Limit Check (Throttle)
    # Count requests in the last 60 seconds
    recent_requests = [t for t in history if now - t < 60]
    
    if len(recent_requests) >= 5:
        logger.warning(f"⏳ IP {ip} throttled (Speed limit). Pausing 15s...")
        time.sleep(15)

    # Log this access
    ip_access_history[ip].append(now)

def call_huggingface(context_json, user_query):
    """Calls HF Inference API with the System Prompt."""
    
    headers = {"Authorization": f"Bearer {HF_TOKEN}"}
    
    # Construct the System Prompt (Moved from Client to Server)
    system_prompt = (
        "You are an expert in linux package management. "
        "Translate user intent to package commands. "
        "Context provides 'is_root'. If False, add 'sudo' where needed. "
        "If True, NO 'sudo'. "
        "Return JSON: {'command': string, 'explanation': string (max 10 words), 'error': string|null}. "
        f"Context: {context_json}"
    )

    # Standard Chat Format for HF Models
    payload = {
        "inputs": f"<start_of_turn>user\n{system_prompt}\nQuery: {user_query}<end_of_turn>\n<start_of_turn>model\n",
        "parameters": {
            "max_new_tokens": 256,
            "temperature": 0.1,
            "return_full_text": False
        }
    }

    try:
        response = requests.post(HF_API_URL, headers=headers, json=payload)
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        logger.error(f"HF API Error: {e}")
        return {"error": "Upstream AI provider failed."}

@app.route('/yups/v1/chat', methods=['POST'])
def chat_handler():
    start_time = time.time()
    client_ip = request.remote_addr
    
    # --- 1. Security Headers Check ---
    client_header = request.headers.get(YUPS_CLIENT_HEADER)
    if client_header != YUPS_CLIENT_SECRET:
        logger.warning(f"🔒 Unauthorized access attempt from {client_ip}")
        abort(403, description="Invalid Client Identification")

    # --- 2. Rate Limiting ---
    check_rate_limit(client_ip)

    # --- 3. Process Request ---
    try:
        data = request.get_json()
        user_query = data.get('query', '')
        context_config = data.get('config', {}) # OS, Distro, PM, is_root
        
        # Log Raw Request
        logger.info(f"📥 [{client_ip}] Query: {user_query} | Context: {json.dumps(context_config)}")

        # Call AI
        hf_response_raw = call_huggingface(json.dumps(context_config), user_query)
        
        # Log Raw Response
        logger.info(f"🤖 HF Response Raw: {json.dumps(hf_response_raw)}")

        # Parse HF Response (Handle list vs dict)
        ai_text = ""
        if isinstance(hf_response_raw, list) and len(hf_response_raw) > 0:
            ai_text = hf_response_raw[0].get('generated_text', '')
        elif isinstance(hf_response_raw, dict):
            ai_text = hf_response_raw.get('generated_text', '')
        
        # Cleanup JSON markdown if present
        if "```json" in ai_text:
            ai_text = ai_text.split("```json")[1].split("```")[0].strip()
        elif "```" in ai_text:
            ai_text = ai_text.split("```")[1].split("```")[0].strip()

        # Parse AI JSON
        try:
            parsed_result = json.loads(ai_text)
        except:
            parsed_result = {"error": "Failed to parse AI response", "raw": ai_text}

    except Exception as e:
        logger.error(f"Internal Error: {e}")
        return jsonify({"error": "Internal Server Error"}), 500

    # --- 4. Stats & Response ---
    duration = time.time() - start_time
    latency_history.append(duration)
    avg_latency = sum(latency_history) / len(latency_history)

    response_payload = {
        "result": parsed_result,
        "meta": {
            "server_time": duration,
            "avg_latency_10": avg_latency,
            "model": HF_MODEL_ID
        }
    }

    return jsonify(response_payload)

@app.route('/health', methods=['GET'])
def health_check():
    return jsonify({"status": "ok", "yups_version": "api-v1"}), 200

if __name__ == '__main__':
    # Run dev server
    app.run(host='0.0.0.0', port=5000)
