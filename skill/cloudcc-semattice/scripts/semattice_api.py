#!/usr/bin/env python3
"""登录并调用 Semattice 能力，同时避免泄露 Bearer 令牌。"""

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
from typing import Any, Callable

from semattice_auth import (
    AuthError,
    AuthManager,
    AuthSettings,
    DEFAULT_CAPABILITY_SCOPES,
    open_without_redirect,
    validate_service_url,
)


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
            .replace("positional arguments:", "命令:", 1)
        )


def add_call_arguments(parser: argparse.ArgumentParser) -> None:
    parser.add_argument(
        "--base-url", default=os.environ.get("SEMATTICE_BASE_URL"), help="Semattice 服务根地址"
    )
    parser.add_argument("--capability", required=True, help="要调用的能力 ID")
    source = parser.add_mutually_exclusive_group(required=True)
    source.add_argument("--input", help="以单个 JSON 对象表示的能力输入")
    source.add_argument("--input-file", type=Path, help="包含 JSON 输入对象的文件路径")
    parser.add_argument("--request-id", help="请求标识符；省略时自动生成")
    parser.add_argument("--idempotency-key", help="同一逻辑写操作使用的稳定幂等键")
    parser.add_argument("--token-env", default="SEMATTICE_TOKEN", help="保存短期令牌的环境变量名")
    parser.add_argument("--credentials-file", type=Path, help="登录缓存路径；默认使用用户配置目录")
    parser.add_argument("--timeout", type=float, default=30.0, help="请求超时秒数")
    parser.add_argument("--dry-run", action="store_true", help="只输出请求地址和正文，不发起调用")


def parse_args(argv: list[str] | None = None) -> argparse.Namespace:
    """Parse the legacy capability invocation syntax."""
    parser = ChineseArgumentParser(description="调用 Semattice 能力 API 端点", add_help=False)
    parser.add_argument("-h", "--help", action="help", help="显示帮助信息并退出")
    add_call_arguments(parser)
    return parser.parse_args(argv)


def command_parser() -> ChineseArgumentParser:
    parser = ChineseArgumentParser(description="登录并调用 CloudCC Semattice", add_help=False)
    parser.add_argument("-h", "--help", action="help", help="显示帮助信息并退出")
    commands = parser.add_subparsers(dest="command", required=True, title="命令")

    login = commands.add_parser("login", help="通过 Keycloak Authorization Code + PKCE 登录")
    login.add_argument("--issuer", help="Keycloak Realm issuer")
    login.add_argument("--client-id", help="Keycloak public client ID")
    login.add_argument("--base-url", help="Semattice 服务根地址")
    login.add_argument("--company-id", help="要登录的 Keycloak Organization alias")
    login.add_argument(
        "--scope",
        action="append",
        default=list(DEFAULT_CAPABILITY_SCOPES),
        help="额外请求的 Semattice scope；可重复，默认请求全部已发布能力所需 scope",
    )
    login.add_argument("--credentials-file", type=Path, help="登录缓存路径；默认使用用户配置目录")
    login.add_argument("--login-timeout", type=float, default=180.0, help="等待浏览器回调的秒数")
    login.add_argument("--no-browser", action="store_true", help="不自动打开浏览器，只显示登录地址")

    status = commands.add_parser("status", help="显示登录状态，不输出令牌")
    status.add_argument("--credentials-file", type=Path, help="登录缓存路径；默认使用用户配置目录")

    logout = commands.add_parser("logout", help="删除系统凭据和本地短期登录缓存")
    logout.add_argument("--issuer", help="缓存缺失时用于定位系统凭据的 Keycloak issuer")
    logout.add_argument("--client-id", help="缓存缺失时用于定位系统凭据的 client ID")
    logout.add_argument("--credentials-file", type=Path, help="登录缓存路径；默认使用用户配置目录")

    call = commands.add_parser("call", help="调用一个 Semattice Capability")
    add_call_arguments(call)
    return parser


def load_input(args: argparse.Namespace) -> dict[str, Any]:
    if args.input is not None:
        raw = args.input
    else:
        raw = args.input_file.read_text(encoding="utf-8")
    value = json.loads(raw)
    if not isinstance(value, dict):
        raise ValueError("能力输入必须是 JSON 对象")
    return value


def validate_base_url(raw: str | None) -> str:
    if not raw:
        raise ValueError("请设置 SEMATTICE_BASE_URL、传入 --base-url，或先执行 semattice login")
    try:
        return validate_service_url(raw, "Semattice 服务根地址")
    except AuthError as exc:
        raise ValueError(str(exc)) from exc


def build_request(args: argparse.Namespace, capability_input: dict[str, Any]) -> tuple[str, dict[str, Any]]:
    request_id = args.request_id or f"req-{uuid.uuid4()}"
    body: dict[str, Any] = {"request_id": request_id, "input": capability_input}
    if args.idempotency_key:
        body["idempotency_key"] = args.idempotency_key
    encoded_capability = urllib.parse.quote(args.capability, safe=".-_")
    return encoded_capability, body


def invoke_once(
    url: str,
    body: dict[str, Any],
    token: str,
    timeout: float,
    *,
    urlopen: Callable[..., Any] = open_without_redirect,
) -> tuple[int, dict[str, Any]]:
    request = urllib.request.Request(
        url,
        data=json.dumps(body, separators=(",", ":")).encode("utf-8"),
        method="POST",
        headers={"Authorization": f"Bearer {token}", "Content-Type": "application/json"},
    )
    try:
        with urlopen(request, timeout=timeout) as response:
            status_code = response.status
            raw_response = response.read()
    except urllib.error.HTTPError as exc:
        status_code = exc.code
        try:
            raw_response = exc.read()
        except OSError:
            raw_response = b""
    try:
        payload = json.loads(raw_response)
    except json.JSONDecodeError:
        payload = {"status": "failed", "error": "服务器返回了非 JSON 响应", "http_status": status_code}
    if not isinstance(payload, dict):
        payload = {"status": "failed", "error": "服务器返回的 JSON 不是对象", "http_status": status_code}
    return status_code, payload


def invoke_with_auth_retry(
    url: str,
    body: dict[str, Any],
    token: str,
    timeout: float,
    *,
    refresh: Callable[[], str] | None = None,
    urlopen: Callable[..., Any] = open_without_redirect,
) -> tuple[int, dict[str, Any]]:
    status_code, payload = invoke_once(url, body, token, timeout, urlopen=urlopen)
    if status_code == 401 and refresh is not None:
        status_code, payload = invoke_once(url, body, refresh(), timeout, urlopen=urlopen)
    return status_code, payload


def auth_manager(args: argparse.Namespace) -> AuthManager:
    return AuthManager(args.credentials_file if getattr(args, "credentials_file", None) else None)


def run_call(args: argparse.Namespace) -> int:
    try:
        capability_input = load_input(args)
        encoded_capability, body = build_request(args, capability_input)
    except (OSError, ValueError, json.JSONDecodeError) as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 2

    manager: AuthManager | None = None
    cached_session = None
    explicit_token = os.environ.get(args.token_env, "").strip()
    try:
        if args.dry_run and args.base_url:
            base_url = validate_base_url(args.base_url)
            token = ""
        elif explicit_token:
            base_url = validate_base_url(args.base_url)
            token = explicit_token
        else:
            manager = auth_manager(args)
            if args.dry_run:
                cached_session = manager.cache.load()
            else:
                cached_session = manager.get_session()
            base_url = validate_base_url(args.base_url or cached_session.semattice_base_url)
            token = cached_session.oact
    except (AuthError, ValueError) as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 2

    url = f"{base_url}/v1/capabilities/{encoded_capability}/invoke"
    if args.dry_run:
        print(json.dumps({"url": url, "body": body}, ensure_ascii=False, indent=2, sort_keys=True))
        return 0

    try:
        refresh = None
        if manager is not None:
            refresh = lambda: manager.get_session(force_refresh=True).oact
        status_code, payload = invoke_with_auth_retry(
            url, body, token, args.timeout, refresh=refresh
        )
    except urllib.error.URLError as exc:
        print(json.dumps({"status": "failed", "error": str(exc.reason)}, ensure_ascii=False), file=sys.stderr)
        return 1
    except AuthError as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 1

    print(json.dumps(payload, ensure_ascii=False, indent=2, sort_keys=True))
    return 0 if 200 <= status_code < 300 and payload.get("status") == "succeeded" else 1


def run_login(args: argparse.Namespace) -> int:
    try:
        settings = AuthSettings.from_values(
            issuer=args.issuer,
            client_id=args.client_id,
            semattice_base_url=args.base_url,
            scopes=tuple(args.scope),
        )
        session = auth_manager(args).login(
            settings,
            requested_company=args.company_id,
            no_browser=args.no_browser,
            timeout=args.login_timeout,
        )
    except AuthError as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 1
    print(
        json.dumps(
            {
                "status": "authenticated",
                "company_id": session.company_id,
                "semattice_base_url": session.semattice_base_url,
                "oact_expires_at": session.oact_expires_at,
                "scope_count": len(session.scopes),
                "scopes": list(session.scopes),
            },
            ensure_ascii=False,
            indent=2,
            sort_keys=True,
        )
    )
    return 0


def run_status(args: argparse.Namespace) -> int:
    try:
        value = auth_manager(args).status()
    except AuthError as exc:
        print(json.dumps({"status": "not_authenticated", "error": str(exc)}, ensure_ascii=False))
        return 1
    print(json.dumps(value, ensure_ascii=False, indent=2, sort_keys=True))
    return 0


def run_logout(args: argparse.Namespace) -> int:
    try:
        settings = AuthSettings.from_values(issuer=args.issuer, client_id=args.client_id)
        auth_manager(args).logout(settings)
    except AuthError as exc:
        print(json.dumps({"status": "failed", "error": str(exc)}, ensure_ascii=False), file=sys.stderr)
        return 1
    print(json.dumps({"status": "logged_out"}, ensure_ascii=False))
    return 0


def command_main(argv: list[str] | None = None) -> int:
    args = command_parser().parse_args(argv)
    if args.command == "login":
        return run_login(args)
    if args.command == "status":
        return run_status(args)
    if args.command == "logout":
        return run_logout(args)
    return run_call(args)


def main(argv: list[str] | None = None) -> int:
    arguments = list(sys.argv[1:] if argv is None else argv)
    if arguments and arguments[0] in {"login", "status", "logout", "call"}:
        return command_main(arguments)
    return run_call(parse_args(arguments))


if __name__ == "__main__":
    raise SystemExit(main())
