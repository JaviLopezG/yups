#!/usr/bin/env python3
"""
inspect-session.py - Interactive step-by-step YUPS session log inspector.

Allows listing past sessions (defaulting to the latest), navigating step-by-step
through each turn/event:
  - n / Enter / Right: Next step
  - p / b / Left: Previous step
  - Up (↑) / Down (↓): Scroll up / Scroll down within current step
  - t: Go to first step (top)
  - e: Go to last step (end)
  - 1-N: Jump directly to step number
  - s: Select another session
  - q: Quit
"""

import glob
import json
import os
import re
import shutil
import sys
from typing import Any, Dict, List, Optional, Tuple


# ANSI Color Codes
USE_COLOR = sys.stdout.isatty() and os.environ.get("NO_COLOR") is None

def color(text: str, code: str) -> str:
    if not USE_COLOR:
        return text
    return f"\033[{code}m{text}\033[0m"

def orange(text: str) -> str:
    return color(text, "38;5;214")

def cyan(text: str) -> str:
    return color(text, "36")

def green(text: str) -> str:
    return color(text, "32")

def yellow(text: str) -> str:
    return color(text, "33")

def red(text: str) -> str:
    return color(text, "31")

def bold(text: str) -> str:
    return color(text, "1")

def dim(text: str) -> str:
    return color(text, "2")


def get_terminal_size() -> Tuple[int, int]:
    try:
        sz = shutil.get_terminal_size((80, 24))
        return sz.columns, sz.lines
    except Exception:
        return 80, 24


def hr(char: str = "=", col: Optional[str] = None) -> str:
    cols, _ = get_terminal_size()
    w = min(cols, 100)
    line = char * w
    if col:
        return color(line, col)
    return line


def decode_escaped_string(s: str) -> str:
    """Decode escaped characters in JSON string content (such as \\n, \\t, unicode)."""
    if not s:
        return ""
    try:
        if s.startswith('"') and s.endswith('"'):
            return json.loads(s)
        return json.loads(f'"{s}"')
    except Exception:
        s = s.replace("\\n", "\n").replace("\\t", "\t").replace('\\"', '"').replace("\\\\", "\\")
        s = s.replace("\\u0026", "&").replace("\\u003c", "<").replace("\\u003e", ">")
        return s


class SessionInfo:
    def __init__(self, filepath: str, session_id: str = "", timestamp: str = "",
                 pid: str = "", cmd: str = "", model: str = "", turns: int = 0,
                 status: str = "", duration: str = ""):
        self.filepath = filepath
        self.session_id = session_id or os.path.basename(filepath).replace("session-", "").replace(".log", "")
        self.timestamp = timestamp
        self.pid = pid
        self.cmd = cmd
        self.model = model
        self.turns = turns
        self.status = status
        self.duration = duration


class Step:
    def __init__(self, title: str, kind: str, content: str, turn: Optional[int] = None):
        self.title = title
        self.kind = kind
        self.content = content
        self.turn = turn


def resolve_logs_dir() -> str:
    """Locate the logs directory in standard locations."""
    candidates = []
    if "YUPS_DIR" in os.environ:
        candidates.append(os.path.join(os.environ["YUPS_DIR"], "logs"))
    home = os.path.expanduser("~")
    candidates.append(os.path.join(home, ".yups", "logs"))
    candidates.append(os.path.join(os.getcwd(), ".yups", "logs"))

    for c in candidates:
        if os.path.isdir(c):
            return c
    return candidates[1]


def discover_sessions(logs_dir: str) -> List[SessionInfo]:
    """Discover all session logs, reading metadata from sessions.log if available."""
    sessions_map: Dict[str, SessionInfo] = {}

    summary_file = os.path.join(logs_dir, "sessions.log")
    if os.path.isfile(summary_file):
        try:
            with open(summary_file, "r", encoding="utf-8", errors="replace") as f:
                for line in f:
                    line = line.strip()
                    if not line or not line.startswith("["):
                        continue
                    ts_match = re.search(r"^\[([^\]]+)\]", line)
                    ts = ts_match.group(1) if ts_match else ""
                    id_match = re.search(r"id=([^\s]+)", line)
                    sid = id_match.group(1) if id_match else ""
                    pid_match = re.search(r"pid=([^\s]+)", line)
                    pid = pid_match.group(1) if pid_match else ""
                    cmd_match = re.search(r'cmd="([^"]*)"', line)
                    cmd = cmd_match.group(1) if cmd_match else ""
                    model_match = re.search(r'model="([^"]*)"', line)
                    model = model_match.group(1) if model_match else ""
                    turns_match = re.search(r"turns=(\d+)", line)
                    turns = int(turns_match.group(1)) if turns_match else 0
                    status_match = re.search(r"status=([^\s]+)", line)
                    status = status_match.group(1) if status_match else ""
                    dur_match = re.search(r"duration=([^\s]+)", line)
                    duration = dur_match.group(1) if dur_match else ""
                    file_match = re.search(r"file=([^\s]+)", line)
                    filename = file_match.group(1) if file_match else f"session-{sid}.log"

                    log_path = os.path.join(logs_dir, filename)
                    if sid:
                        sessions_map[sid] = SessionInfo(
                            filepath=log_path,
                            session_id=sid,
                            timestamp=ts,
                            pid=pid,
                            cmd=cmd,
                            model=model,
                            turns=turns,
                            status=status,
                            duration=duration
                        )
        except Exception:
            pass

    pattern = os.path.join(logs_dir, "session-*.log")
    for fpath in sorted(glob.glob(pattern)):
        base = os.path.basename(fpath)
        sid = base.replace("session-", "").replace(".log", "")
        if sid not in sessions_map:
            ts, cmd, pid = extract_session_header(fpath)
            sessions_map[sid] = SessionInfo(
                filepath=fpath,
                session_id=sid,
                timestamp=ts,
                pid=pid,
                cmd=cmd
            )

    res = list(sessions_map.values())
    res.sort(key=lambda s: (s.timestamp or s.session_id))
    return res


def extract_session_header(filepath: str) -> Tuple[str, str, str]:
    ts, cmd, pid = "", "", ""
    try:
        with open(filepath, "r", encoding="utf-8", errors="replace") as f:
            for _ in range(25):
                line = f.readline()
                if not line:
                    break
                if line.startswith("Timestamp:"):
                    ts = line.split(":", 1)[1].strip()
                elif line.startswith("Command:"):
                    cmd = line.split(":", 1)[1].strip()
                elif line.startswith("PID:"):
                    pid = line.split(":", 1)[1].strip()
    except Exception:
        pass
    return ts, cmd, pid


def format_decoded_json_payload(raw_json: str, is_request: bool = True) -> str:
    """Format and decode JSON payload messages with clean multi-line display."""
    try:
        data = json.loads(raw_json)
    except Exception:
        return raw_json

    out_lines = []
    if is_request:
        model = data.get("model", "")
        out_lines.append(f"{bold('Target Model:')} {cyan(model)}")
        messages = data.get("messages", [])
        for idx, msg in enumerate(messages):
            role = msg.get("role", "")
            raw_content = msg.get("content", "")
            decoded = decode_escaped_string(raw_content)

            role_colored = bold(f"[{role.upper()}]")
            if role == "system":
                role_colored = orange(f"[{role.upper()}]")
            elif role == "user":
                role_colored = green(f"[{role.upper()}]")
            elif role == "assistant":
                role_colored = cyan(f"[{role.upper()}]")
            elif role == "tool":
                role_colored = yellow(f"[{role.upper()}]")

            out_lines.append(f"\n--- Message {idx + 1}: {role_colored} ---")
            if decoded:
                for line in decoded.splitlines():
                    out_lines.append(f"  {line}")

            tool_calls = msg.get("tool_calls", [])
            if tool_calls:
                out_lines.append(f"  {bold('Requested Tool Calls:')}")
                for tc in tool_calls:
                    fn = tc.get("function", {})
                    fn_name = fn.get("name", "")
                    fn_args = fn.get("arguments", {})
                    out_lines.append(f"    - {yellow(fn_name)}: {json.dumps(fn_args, ensure_ascii=False)}")
    else:
        model = data.get("model", "")
        if model:
            out_lines.append(f"{bold('Responding Model:')} {cyan(model)}")
        msg = data.get("message", {})
        role = msg.get("role", "assistant")
        raw_content = msg.get("content", "")
        decoded = decode_escaped_string(raw_content)

        role_colored = cyan(f"[{role.upper()}]")
        out_lines.append(f"\n--- Response: {role_colored} ---")
        if decoded:
            for line in decoded.splitlines():
                out_lines.append(f"  {line}")

        tool_calls = msg.get("tool_calls", [])
        if tool_calls:
            out_lines.append(f"  {bold('Requested Tool Calls:')}")
            for tc in tool_calls:
                fn = tc.get("function", {})
                fn_name = fn.get("name", "")
                fn_args = fn.get("arguments", {})
                out_lines.append(f"    - {yellow(fn_name)}: {json.dumps(fn_args, ensure_ascii=False)}")

    return "\n".join(out_lines)


def parse_session_file(filepath: str) -> List[Step]:
    """Parse a session log file into a series of navigable steps."""
    if not os.path.isfile(filepath):
        return [Step("File Not Found", "error", f"Error: Log file {filepath} does not exist.")]

    with open(filepath, "r", encoding="utf-8", errors="replace") as f:
        full_text = f.read()

    steps: List[Step] = []

    # Step 1: Session Overview & Header
    header_match = re.search(r"YUPS SESSION LOG: ([^\n]+)\n=+\n(.*?)(?=\n--- |\n>>> |\n\[! |\n=+\nSESSION CONCLUSION|\Z)", full_text, re.DOTALL)
    if header_match:
        sid = header_match.group(1).strip()
        body = header_match.group(2).strip()
        steps.append(Step(f"Session Overview ({sid})", "overview", body))

    # Step 2: Local Pipeline Analysis
    local_match = re.search(r"---\s*\[LOCAL PIPELINE ANALYSIS\]\s*---\n(.*?)(?=\n--- |\n>>> |\n\[! |\n=+\nSESSION CONCLUSION|\Z)", full_text, re.DOTALL)
    if local_match:
        steps.append(Step("Local Pipeline Analysis", "local_doc", local_match.group(1).strip()))

    # Step 3: Configuration & Limits
    config_match = re.search(r"---\s*\[CONFIGURATION & LIMITS\]\s*---\n(.*?)(?=\n--- |\n>>> |\n\[! |\n=+\nSESSION CONCLUSION|\Z)", full_text, re.DOTALL)
    if config_match:
        steps.append(Step("Configuration & Limits", "config", config_match.group(1).strip()))

    # Step 4: Target Model Selection
    model_match = re.search(r"---\s*\[TARGET MODEL SELECTION\]\s*---\n(.*?)(?=\n--- |\n>>> |\n\[! |\n=+\nSESSION CONCLUSION|\Z)", full_text, re.DOTALL)
    if model_match:
        steps.append(Step("Target Model Selection", "model", model_match.group(1).strip()))

    # Steps for Turns and Tool executions and Incidents
    pattern = re.compile(
        r"(>>>\s*\[OLLAMA INTERACTION Turn (\d+)\]\s*>>>|"
        r"---\s*\[TOOL EXECUTION Turn (\d+):\s*([^\]]+)\]\s*---|"
        r"\[!\s*(INCIDENT|LIMIT REACHED):\s*([^!]+)\s*!\]|"
        r"=+\s*\nSESSION CONCLUSION\s*\n=+)",
        re.DOTALL
    )

    matches = list(pattern.finditer(full_text))
    for i, m in enumerate(matches):
        start = m.start()
        end = matches[i + 1].start() if i + 1 < len(matches) else len(full_text)
        block = full_text[start:end].strip()

        if m.group(1) and m.group(1).startswith(">>> [OLLAMA INTERACTION"):
            turn_num = int(m.group(2))
            formatted = format_interaction_block(block, turn_num)
            steps.append(Step(f"Ollama Interaction (Turn {turn_num})", "interaction", formatted, turn=turn_num))

        elif m.group(3) and m.group(3).isdigit():
            turn_num = int(m.group(3))
            tool_name = m.group(4).strip()
            clean_block = re.sub(r"^---\s*\[TOOL EXECUTION Turn \d+:[^\]]+\]\s*---\n", "", block)
            clean_block = re.sub(r"\n--- \[END TOOL EXECUTION\] ---$", "", clean_block)
            steps.append(Step(f"Tool Execution: {tool_name} (Turn {turn_num})", "tool", clean_block.strip(), turn=turn_num))

        elif m.group(5) in ("INCIDENT", "LIMIT REACHED"):
            category = m.group(6).strip()
            steps.append(Step(f"Incident / Limit: {category}", "incident", block))

        elif "SESSION CONCLUSION" in block:
            clean_conclusion = re.sub(r"^=+\s*\nSESSION CONCLUSION\s*\n=+\n", "", block)
            clean_conclusion = re.sub(r"\n=+$", "", clean_conclusion)
            steps.append(Step("Session Conclusion", "conclusion", clean_conclusion.strip()))

    if not steps:
        steps.append(Step("Raw Session Log", "raw", full_text))

    return steps


def format_interaction_block(block: str, turn: int) -> str:
    """Extract metadata, request JSON, and response JSON, decoding escaped strings."""
    lines = block.splitlines()
    meta_line = ""
    req_json_lines = []
    resp_json_lines = []
    mode = "meta"

    for line in lines:
        if line.startswith(">>> [OLLAMA INTERACTION"):
            continue
        if line.startswith("Timestamp:"):
            meta_line = line
            continue
        if line.startswith("Request Payload:"):
            mode = "req"
            continue
        if line.startswith("<<< Response Payload:") or line.startswith("<<< ERROR:"):
            mode = "resp"
            continue
        if line.startswith("<<< END INTERACTION <<<"):
            break

        if mode == "req":
            req_json_lines.append(line)
        elif mode == "resp":
            resp_json_lines.append(line)

    res = []
    if meta_line:
        res.append(f"{bold('Interaction Details:')} {meta_line}")

    if req_json_lines:
        req_raw = "\n".join(req_json_lines).strip()
        res.append(f"\n{orange('=== [REQUEST PAYLOAD (Decoded)] ===')}")
        res.append(format_decoded_json_payload(req_raw, is_request=True))

    if resp_json_lines:
        resp_raw = "\n".join(resp_json_lines).strip()
        res.append(f"\n{cyan('=== [RESPONSE PAYLOAD (Decoded)] ===')}")
        res.append(format_decoded_json_payload(resp_raw, is_request=False))

    return "\n".join(res)


def get_styled_step_lines(step: Step) -> List[str]:
    """Generate all formatted lines for a step."""
    content = step.content
    styled: List[str] = []

    if step.kind == "incident":
        for line in content.splitlines():
            styled.append(red(line))
    elif step.kind == "conclusion":
        for line in content.splitlines():
            if "Status:" in line and "SUCCESS" in line:
                styled.append(f"  {green(line)}")
            elif "Status:" in line and ("ERROR" in line or "ABORT" in line or "UNKNOWN" in line):
                styled.append(f"  {red(line)}")
            elif "Suggested Command:" in line:
                styled.append(f"  {orange(bold(line))}")
            elif "Suggested Script:" in line:
                styled.append(f"  {yellow(bold(line))}")
            else:
                styled.append(f"  {line}")
    elif step.kind == "overview":
        for line in content.splitlines():
            if line.startswith("Command:"):
                styled.append(f"  {bold(orange(line))}")
            else:
                styled.append(f"  {line}")
    else:
        for line in content.splitlines():
            styled.append(line)

    return styled


def render_step_view(step: Step, idx: int, total: int, session: SessionInfo, scroll_offset: int = 0) -> Tuple[int, int]:
    """
    Render a step with scroll window support.
    Returns: (total_lines, max_scroll)
    """
    cols, lines = get_terminal_size()
    os.system("clear" if os.name == "posix" else "cls")

    print(hr("=", "38;5;214"))
    header = f"YUPS SESSION: {session.session_id}  |  STEP {idx + 1} of {total}: {step.title}"
    print(bold(header))
    print(hr("=", "38;5;214"))

    styled_lines = get_styled_step_lines(step)
    total_content_lines = len(styled_lines)

    viewport_height = max(5, lines - 6)
    max_scroll = max(0, total_content_lines - viewport_height)
    offset = max(0, min(scroll_offset, max_scroll))

    visible_lines = styled_lines[offset : offset + viewport_height]
    for l in visible_lines:
        print(l)

    if len(visible_lines) < viewport_height:
        for _ in range(viewport_height - len(visible_lines)):
            print()

    print(hr("-", "2"))
    if total_content_lines > viewport_height:
        scroll_pct = int(((offset + len(visible_lines)) / total_content_lines) * 100)
        scroll_info = dim(f" [Lines {offset + 1}-{offset + len(visible_lines)} of {total_content_lines} ({scroll_pct}%) | ↑/↓: scroll]")
    else:
        scroll_info = ""

    prompt_str = f"[{bold('n')}: next | {bold('p')}: prev | {bold('↑/↓')}: scroll | {bold('t')}: first | {bold('e')}: last | {bold('1-' + str(total))}: jump | {bold('s')}: sessions | {bold('q')}: quit]{scroll_info} > "
    sys.stdout.write(prompt_str)
    sys.stdout.flush()

    return total_content_lines, max_scroll


def read_raw_key() -> str:
    """Read a single key or escape sequence cleanly from raw file descriptor."""
    if not sys.stdin.isatty():
        line = sys.stdin.readline()
        if not line:
            return "q"
        return line.strip().lower()

    import tty, termios
    fd = sys.stdin.fileno()
    old_settings = termios.tcgetattr(fd)
    try:
        tty.setraw(fd)
        raw_bytes = os.read(fd, 32)
        if not raw_bytes:
            return "q"

        # Check escape sequences
        if raw_bytes in (b"\x1b[A", b"\x1bOA"):
            return "UP"
        elif raw_bytes in (b"\x1b[B", b"\x1bOB"):
            return "DOWN"
        elif raw_bytes in (b"\x1b[C", b"\x1bOC"):
            return "RIGHT"
        elif raw_bytes in (b"\x1b[D", b"\x1bOD"):
            return "LEFT"
        elif raw_bytes in (b"\x1b[5~",):
            return "PAGE_UP"
        elif raw_bytes in (b"\x1b[6~",):
            return "PAGE_DOWN"
        elif raw_bytes in (b"\x1b[H", b"\x1bOH", b"\x1b[1~"):
            return "HOME"
        elif raw_bytes in (b"\x1b[F", b"\x1bOF", b"\x1b[4~"):
            return "END"
        elif raw_bytes in (b"\x1b",):
            return "ESC"
        elif raw_bytes in (b"\r", b"\n"):
            return "ENTER"
        elif raw_bytes in (b"\x03", b"\x04"): # Ctrl+C, Ctrl+D
            return "q"

        try:
            return raw_bytes.decode("utf-8", errors="replace")
        except Exception:
            return "noop"
    finally:
        termios.tcsetattr(fd, termios.TCSADRAIN, old_settings)


def session_picker_ui(sessions: List[SessionInfo]) -> Optional[SessionInfo]:
    """Display session selection menu with default to the latest."""
    if not sessions:
        print(red("No session logs found."))
        return None

    os.system("clear" if os.name == "posix" else "cls")
    print(hr("=", "38;5;214"))
    print(bold(f"AVAILABLE YUPS SESSIONS ({len(sessions)} found)"))
    print(hr("=", "38;5;214"))

    for i, s in enumerate(sessions):
        num_str = f"[{i + 1}]"
        ts_str = s.timestamp[:19].replace("T", " ") if s.timestamp else s.session_id
        cmd_str = s.cmd if s.cmd else "(no command)"
        if len(cmd_str) > 45:
            cmd_str = cmd_str[:42] + "..."

        status_colored = s.status or "OK"
        if "SUCCESS" in status_colored or "OK" in status_colored:
            status_colored = green(status_colored)
        elif "ERROR" in status_colored or "ABORT" in status_colored or "UNKNOWN" in status_colored:
            status_colored = red(status_colored)

        dur_str = f"({s.duration})" if s.duration else ""
        turns_str = f"{s.turns} turns" if s.turns else ""
        meta = " ".join(filter(None, [s.model, turns_str, dur_str, status_colored]))

        latest_mark = orange(" (latest)") if i == len(sessions) - 1 else ""
        print(f"  {bold(num_str):<5} {ts_str} | {meta} | {cyan(cmd_str)}{latest_mark}")

    print(hr("-", "2"))
    default_idx = len(sessions)
    prompt = f"Select session [1-{len(sessions)}, {bold('Default: ' + str(default_idx))} (latest)] (or 'q' to quit): "

    while True:
        try:
            choice = input(prompt).strip().lower()
        except (KeyboardInterrupt, EOFError):
            print()
            return None

        if choice in ("q", "quit", "exit"):
            return None
        if choice == "":
            return sessions[-1]
        if choice.isdigit():
            val = int(choice)
            if 1 <= val <= len(sessions):
                return sessions[val - 1]
        print(red(f"Please enter a number between 1 and {len(sessions)}, or 'q' to quit."))


def inspect_session(session: SessionInfo, sessions_list: List[SessionInfo]) -> None:
    """Interactive loop stepping through the steps of a session log."""
    steps = parse_session_file(session.filepath)
    if not steps:
        print(red("No steps found in session log."))
        return

    current_idx = 0
    scroll_offset = 0
    total = len(steps)

    while True:
        step = steps[current_idx]
        total_lines, max_scroll = render_step_view(step, current_idx, total, session, scroll_offset)

        key = read_raw_key()

        if key in ("q", "Q"):
            break
        elif key in ("n", "N", "RIGHT", "ENTER"):
            if current_idx < total - 1:
                current_idx += 1
                scroll_offset = 0
        elif key in ("p", "P", "b", "B", "LEFT"):
            if current_idx > 0:
                current_idx -= 1
                scroll_offset = 0
        elif key == "UP":
            scroll_offset = max(0, scroll_offset - 3)
        elif key == "DOWN":
            scroll_offset = min(max_scroll, scroll_offset + 3)
        elif key == "PAGE_UP":
            scroll_offset = max(0, scroll_offset - 10)
        elif key in ("PAGE_DOWN", " "):
            scroll_offset = min(max_scroll, scroll_offset + 10)
        elif key in ("t", "T", "HOME"):
            current_idx = 0
            scroll_offset = 0
        elif key in ("e", "E", "END"):
            current_idx = total - 1
            scroll_offset = 0
        elif key in ("s", "S"):
            new_s = session_picker_ui(sessions_list)
            if new_s:
                session = new_s
                steps = parse_session_file(session.filepath)
                total = len(steps)
                current_idx = 0
                scroll_offset = 0
        elif key.isdigit():
            val = int(key)
            if total <= 9:
                if 1 <= val <= total:
                    current_idx = val - 1
                    scroll_offset = 0
            else:
                sys.stdout.write(f"\nJump to step [1-{total}]: {val}")
                sys.stdout.flush()
                try:
                    rest = input().strip()
                    full_str = str(val) + rest
                    if full_str.isdigit():
                        j_val = int(full_str)
                        if 1 <= j_val <= total:
                            current_idx = j_val - 1
                            scroll_offset = 0
                except (KeyboardInterrupt, EOFError):
                    pass


def main() -> int:
    logs_dir = resolve_logs_dir()

    target_session: Optional[SessionInfo] = None
    sessions: List[SessionInfo] = []

    if len(sys.argv) > 1:
        arg = sys.argv[1]
        if arg in ("-h", "--help", "help"):
            print("Usage: inspect-session.py [SESSION_ID | /path/to/session.log]")
            print("\nNavigation Keys:")
            print("  n / Enter     Next step")
            print("  p / b         Previous step")
            print("  ↑ (Up)        Scroll up within step")
            print("  ↓ (Down)      Scroll down within step")
            print("  t             Go to first step (top)")
            print("  e             Go to last step (end)")
            print("  1-N           Jump directly to step number")
            print("  s             Select another session")
            print("  q             Quit")
            return 0

        if os.path.isfile(arg):
            target_session = SessionInfo(filepath=os.path.abspath(arg))
        else:
            candidate = os.path.join(logs_dir, f"session-{arg}.log")
            if os.path.isfile(candidate):
                target_session = SessionInfo(filepath=candidate, session_id=arg)
            else:
                candidate2 = os.path.join(logs_dir, arg)
                if os.path.isfile(candidate2):
                    target_session = SessionInfo(filepath=candidate2)

    sessions = discover_sessions(logs_dir)

    if target_session is None:
        target_session = session_picker_ui(sessions)

    if target_session is None:
        return 0

    inspect_session(target_session, sessions)
    return 0


if __name__ == "__main__":
    sys.exit(main())
