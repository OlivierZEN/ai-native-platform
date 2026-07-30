#!/usr/bin/env python3
"""调用一个 Semattice 能力，同时避免泄露 Bearer 令牌。"""

from __future__ import annotations

import argparse
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request
import uuid
from pathlib import Path


class ChineseArgumentParser(argparse.ArgumentParser):
    """将参数解析器自动生成的帮助标题转换为中文。"""

    def format_usage(self) -> str:
        return super().format_usage().replace("usage:", "用法:", 1)

    def format_help(self) -> str:
        return (
            super()
            .format_help()
            .replace("usage:", "用法:", 1)
            .replace("optional arguments:", "可选参数:", 1)
            .replace("options:", "可选参数:", 1)
        )


def parse_args() -> argparse.Namespace:
    parser = ChineseArgumentParser(description="调用 Semattice 能力 API 端点", add_help=False)
    parser.add_argument("-h", "--help", action="help", help="显示帮助信息并退出")
    parser.add_argument("--base-url", default=os.environ.get("SEMATTICE_BASE_URL"), help="Semattice 服务根地址")
    parser.add_argument("--capability", required=True, help="要调用的能力 ID")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--input", help="以单个 JSON 对象表示的能力输入")
    source.add_argument("--input-file", type=Path, help="包含 JSON 输入对象的文件路径")
    parser.add_argument("--request-id", help="请求标识符；省略时自动生成")
    parser.add_argument("--idempotency-key", help="同一逻辑写操作使用的稳定幂等键")
    parser.add_argument("--token-env", default="SEMATTICE_TOKEN", help="保存短期令牌的环境变量名")
    parser.add_argument("--timeout", type=float, default=30.0, help="请求超时秒数")
    parser.add_argument("--dry-run", action="store_true", help="只输出请求地址和正文，不发起调用")
    return parser.parse_args()


def load_input(args: argparse.Namespace) -> dict:
    raw = args.input if args.input is not None else args.input_file.read_text(encoding="utf-8")
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise ValueError("能力输入必须是 JSON 对象")
    return value


def validate_base_url(raw: str | None) -> str:
    if not raw:
        raise ValueError("请设置 SEMATTICE_BASE_URL 或传入 --base-url")
    base_url = raw.rstrip("/")
    parsed = urllib.parse.urlparse(base_url)
    if parsed.scheme not in {"http", "https"} or not parsed.netloc:
        raise ValueError("服务根地址必须是绝对 http(s) URL")
    if parsed.scheme == "http" and parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise ValueError("明文 HTTP 仅允许用于本机回环开发环境")
    return base_url


def main() -> int:
    args = parse_args()
    try:
        base_url = validate_base_url(args.base_url)
        capability_input = load_input(args)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 2

    request_id = args.request_id or f"req-{uuid.uuid4()}"
    body = {"request_id": request_id, "input": capability_input}
    if args.idempotency_key:
        body["idempotency_key"] = args.idempotency_key

    encoded_capability = urllib.parse.quote(args.capability, safe=".-_")
    url = f"{base_url}/v1/capabilities/{encoded_capability}/invoke"
    if args.dry_run:
        print(json.dumps({"url": url, "body": body}, ensure_ascii=False, indent=2, sort_keys=True))
        return 0

    token = os.environ.get(args.token_env, "").strip()
    if not token:
        print(
            json.dumps(
                {"status": "failed", "error": f"请在 {args.token_env} 中设置短期令牌"},
                ensure_ascii=False,
            ),
            file=sys.stderr,
        )
        return 2

    request = urllib.request.Request(
        url,
        data=json.dumps(body, separators=(",", ":")).encode("utf-8"),
        method="POST",
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    status_code = 0
    try:
        with urllib.request.urlopen(request, timeout=args.timeout) as response:
            status_code = response.status
            raw_response = response.read()
    except urllib.error.HTTPError as exc:
        status_code = exc.code
        raw_response = exc.read()
    except urllib.error.URLError as exc:
        print(json.dumps({"status": "failed", "error": str(exc.reason)}, ensure_ascii=False), file=sys.stderr)
        return 1

    try:
        payload = json.loads(raw_response)
    except json.JSONDecodeError:
        payload = {"status": "failed", "error": "服务器返回了非 JSON 响应", "http_status": status_code}
    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))
    return 0 if 200 <= status_code < 300 and payload.get("status") == "succeeded" else 1


if __name__ == "__main__":
    raise SystemExit(main())
