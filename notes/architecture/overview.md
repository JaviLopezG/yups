# System Architecture & Component Overview

**YUPS** is structured as a "monorepo" containing multiple distinct but related components. This document outlines the purpose and interactions of each part of the system.

## 🧩 Components Breakdown

### 1. The Straw-Boss (Core Logic)
Currently exists in two forms:
*   **🐍 Python MVP (`/yups`)**: The current working prototype. It is a Python script that acts as a wrapper for package managers.
    *   **Features**: Command Not Found (CNF) handler, Command Error (CE) handler, Package Manager translation.
    *   **Integration**: Connects to the Trillian API for AI-powered suggestions.
*   **🐹 Go CLI (`/cli`)**: The **next-generation client**.
    *   **Goal**: To be a single, static binary (zero-dependency) for easy distribution.
    *   **Key Tech**: Uses `llama.cpp` (via C bindings) to potentially run local inference, reducing dependency on the online API for basic tasks.

### 2. Trillian API (`/server`)
The backend brain of the operation.
*   **Tech Stack**: Python (Flask).
*   **Role**: Processes natural language queries from the client and translates them into terminal commands.
*   **AI Backend Strategy**:
    *   **Primary (On-Premise)**: Connects to **"Marvin"** (a local server at `100.70.90.66`) running **Ollama** with `gemma3:27b-it-chat`.
    *   **Fallback (Cloud)**: Connects to **Hugging Face Inference API** (`google/gemma-2-27b-it`) if Marvin is unreachable.
*   **Security**: Implements IP-based rate limiting and simple header-based authentication (`X-Yups-Client-Version`).

### 3. Web Presence
*   **Landing Page (`/web`)**:
    *   A static HTML site (`yups.io`) introducing the product.
    *   "The Universal Prompt Straw-Boss" branding.
*   **Status Page (`/status-web`)**:
    *   A status dashboard (likely `status.yups.io`) to monitor system health (Marvin uptime, API latency).

### 4. Testing & QA (`/testing_environment`)
*   **Infrastructure**: Docker Compose based.
*   **Purpose**: Validates YUPS against different Linux distributions (Fedora, Ubuntu, Arch, etc.) to ensure the "Polyglot" promise works.

---

## 📡 Connectivity Flow

```mermaid
graph TD
    User[User Terminal] -->|Config/CNF| Client[YUPS Client (Python/Go)]
    Client -->|API Request| API[Trillian API (Flask)]
    
    subgraph "AI Inference Layer"
        API -->|Primary| Marvin[🏠 Marvin (Ollama/Local)]
        API -->|Fallback| HF[☁️ Hugging Face API]
    end
    
    Client -->|Package Mgmt| LocalOS[Local OS (apt/dnf/pacman)]
```
