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
# Default to the model you found working well
HF_MODEL_ID = os.environ.get("HF_MODEL_ID", "google/gemma-2-27b-it") 
# NEW ENDPOINT (OpenAI Compatible Router)
HF_API_URL = "https://router.huggingface.co/v1/chat/completions"

# Security
YUPS_CLIENT_HEADER = os.environ.get("YUPS_CLIENT_HEADER", "X-Yups-Client-Version")
YUPS_CLIENT_SECRET = os.environ.get("YUPS_CLIENT_SECRET", "yups-v1-secret-key")

# --- LOGGING SETUP ---
LOG_DIR = "logs"
os.makedirs(LOG_DIR, exist_ok=True)
LOG_FILE = os.path.join(LOG_DIR, "server_requests.log")

logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s | %(levelname)s | %(message)s',
    handlers=[
        logging.FileHandler(LOG_FILE),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger("YupsAPI")

# --- IN-MEMORY STATE ---
ip_access_history = defaultdict(list)
latency_history = deque(maxlen=10)

def clean_old_timestamps(ip):
    now = time.time()
    ip_access_history[ip] = [t for t in ip_access_history[ip] if now - t < 3600]

def check_rate_limit(ip):
    clean_old_timestamps(ip)
    now = time.time()
    history = ip_access_history[ip]
    
    if len(history) >= 100:
        logger.warning(f"⛔ IP {ip} blocked (Hourly limit exceeded).")
        abort(429, description="Hourly rate limit exceeded (100 req/h).")

    recent_requests = [t for t in history if now - t < 60]
    
    if len(recent_requests) >= 5:
        logger.warning(f"⏳ IP {ip} throttled (Speed limit). Pausing 15s...")
        time.sleep(15)

    ip_access_history[ip].append(now)

def call_huggingface(context_json, user_query):
    """Calls HF Inference API using the new OpenAI-compatible format."""
    
    headers = {
        "Authorization": f"Bearer {HF_TOKEN}",
        "Content-Type": "application/json"
    }
    
    # Construct the System Instructions
    # Note: Some models via router prefer 'system' role, others merge it.
    # Putting it in 'user' or 'system' usually works. We'll use 'system' role 
    # as it's the semantic standard for the new API.
    system_content = (
        "You are an expert in linux package management. "
        "Translate user intent to package commands. "
        "Context provides 'is_root'. If False, add 'sudo' where needed. "
        "If True, NO 'sudo'. "
        "Return ONLY a JSON with fields: 'command' (string), 'explanation' (string, max 10 words), 'error' (string|null). "
        f"Context: {context_json}"
    )

    # OpenAI Compatible Payload
    payload = {
        "model": HF_MODEL_ID,
        "messages": [
            {
                "role": "system",
                "content": system_content
            },
            {
                "role": "user",
                "content": user_query
            }
        ],
        "max_tokens": 500,    # 'max_new_tokens' is now 'max_tokens'
        "temperature": 0.1,
        "stream": False
    }

    try:
        response = requests.post(HF_API_URL, headers=headers, json=payload)
        
        # Debug logging if something goes wrong
        if not response.ok:
            logger.error(f"HF Error Body: {response.text}")
            
        response.raise_for_status()
        return response.json()
    except requests.exceptions.RequestException as e:
        logger.error(f"HF API Error: {e}")
        return {"error": "Upstream AI provider failed."}

@app.route('/yups/v1/chat', methods=['POST'])
def chat_handler():
    start_time = time.time()
    client_ip = request.remote_addr
    
    # 1. Security Headers Check
    client_header = request.headers.get(YUPS_CLIENT_HEADER)
    if client_header != YUPS_CLIENT_SECRET:
        logger.warning(f"🔒 Unauthorized access attempt from {client_ip}")
        abort(403, description="Invalid Client Identification")

    # 2. Rate Limiting
    check_rate_limit(client_ip)

    # 3. Process Request
    try:
        data = request.get_json()
        user_query = data.get('query', '')
        context_config = data.get('config', {}) 
        
        logger.info(f"📥 [{client_ip}] Query: {user_query} | Context: {json.dumps(context_config)}")

        hf_response_raw = call_huggingface(json.dumps(context_config), user_query)
        
        # Log Full Response for Debugging
        logger.info(f"🤖 HF Response Raw: {json.dumps(hf_response_raw)}")

        ai_text = ""
        
        # --- NEW PARSING LOGIC (OpenAI Format) ---
        # Structure: { "choices": [ { "message": { "content": "..." } } ] }
        if "choices" in hf_response_raw and len(hf_response_raw["choices"]) > 0:
            message = hf_response_raw["choices"][0].get("message", {})
            ai_text = message.get("content", "")
        elif "error" in hf_response_raw:
            # Propagate internal error
            return jsonify({"error": hf_response_raw["error"]}), 502
        
        # Cleanup JSON markdown if present
        if "```json" in ai_text:
            ai_text = ai_text.split("```json")[1].split("```")[0].strip()
        elif "```" in ai_text:
            ai_text = ai_text.split("```")[1].split("```")[0].strip()

        # Parse AI JSON
        try:
            parsed_result = json.loads(ai_text)
        except:
            logger.warning(f"⚠️ Failed to parse JSON. Raw AI text: {ai_text}")
            parsed_result = {"error": "Failed to parse AI response", "raw": ai_text}

    except Exception as e:
        logger.error(f"Internal Error: {e}")
        return jsonify({"error": "Internal Server Error"}), 500

    # 4. Stats & Response
    duration = time.time() - start_time
    latency_history.append(duration)
    avg_latency = sum(latency_history) / len(latency_history) if latency_history else 0

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
    return jsonify({"status": "ok", "yups_version": "api-v1-router"}), 200

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
