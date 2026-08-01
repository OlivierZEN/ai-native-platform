#!/usr/bin/env python3
"""Keycloak PKCE login and short-lived OACT management for Semattice."""

from __future__ import annotations

import base64
import ctypes
import hashlib
import json
import os
import platform
import re
import secrets
import shutil
import stat
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request
import webbrowser
from dataclasses import asdict, dataclass
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from typing import Any, Callable, Protocol


DEFAULT_ISSUER = "https://sso.agentcici.com/realms/agentcici"
DEFAULT_CLIENT_ID = "semattice-cli"
DEFAULT_SEMATTICE_BASE_URL = "https://semattice.agentcici.com"
DEFAULT_TOKEN_PATH = "/v1/auth/token"
SESSION_CACHE_VERSION = 3
DEFAULT_CAPABILITY_SCOPES = (
    "system.capability.read",
    "tenant.status.read",
    "tenant.lifecycle.write",
    "tenant.entitlement.write",
    "tenant.decommission",
    "metadata.version.write",
    "metadata.definition.write",
    "metadata.publish",
    "metadata.read",
    "metadata.changeset.write",
    "metadata.changeset.read",
    "metadata.changeset.approve",
    "metadata.changeset.publish",
    "metadata.changeset.execute",
    "metadata.changeset.purge",
    "metadata.changeset.rollback",
    "usage.read",
    "usage.platform.read",
    "runtime.record.create",
    "runtime.record.read",
    "runtime.record.update",
    "runtime.record.delete",
    "authorization.manage",
    "record.share.manage",
    "organization.manage",
    "authorization.read",
)
KEYCHAIN_SERVICE = "cloudcc-semattice"
TOKEN_SKEW_SECONDS = 60
SAFE_ERROR_CODE = re.compile(r"^[A-Za-z0-9_.-]{1,64}$")


class AuthError(RuntimeError):
    """An authentication failure safe to show without credentials."""


class NoRedirectHandler(urllib.request.HTTPRedirectHandler):
    """Reject redirects so Authorization headers never cross origins."""

    def redirect_request(self, req: Any, fp: Any, code: int, msg: str, headers: Any, newurl: str) -> None:
        del req, fp, code, msg, headers, newurl
        return None


def open_without_redirect(request: urllib.request.Request, *, timeout: float) -> Any:
    return urllib.request.build_opener(NoRedirectHandler()).open(request, timeout=timeout)


def _base64url(raw: bytes) -> str:
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def generate_pkce() -> tuple[str, str]:
    verifier = _base64url(secrets.token_bytes(64))
    challenge = _base64url(hashlib.sha256(verifier.encode("ascii")).digest())
    return verifier, challenge


def jwt_exp(token: str) -> int | None:
    """Read an unverified exp only for local cache timing, never for authorization."""
    try:
        payload = token.split(".")[1]
        payload += "=" * (-len(payload) % 4)
        value = json.loads(base64.urlsafe_b64decode(payload.encode("ascii")))
        exp = value.get("exp")
        return int(exp) if isinstance(exp, (int, float)) else None
    except (IndexError, ValueError, TypeError, json.JSONDecodeError):
        return None


def validate_service_url(raw: str, label: str) -> str:
    value = raw.rstrip("/")
    parsed = urllib.parse.urlparse(value)
    if not parsed.netloc or parsed.scheme not in {"http", "https"}:
        raise AuthError(f"{label}必须是绝对 http(s) URL")
    if parsed.scheme == "http" and parsed.hostname not in {"127.0.0.1", "localhost", "::1"}:
        raise AuthError(f"{label}仅允许 HTTPS；明文 HTTP 只可用于本机回环开发")
    if parsed.username or parsed.password:
        raise AuthError(f"{label}不得包含用户凭据")
    if parsed.query or parsed.fragment:
        raise AuthError(f"{label}不得包含 query 或 fragment")
    return value


def default_credentials_file() -> Path:
    configured = os.environ.get("SEMATTICE_CREDENTIALS_FILE", "").strip()
    if configured:
        return Path(configured).expanduser()
    config_root = Path(os.environ.get("XDG_CONFIG_HOME", "")).expanduser() if os.environ.get("XDG_CONFIG_HOME") else Path.home() / ".config"
    return config_root / "cloudcc-semattice" / "credentials.json"


@dataclass(frozen=True)
class AuthSettings:
    issuer: str
    client_id: str
    semattice_base_url: str
    scopes: tuple[str, ...] = ()

    @classmethod
    def from_values(
        cls,
        *,
        issuer: str | None = None,
        client_id: str | None = None,
        semattice_base_url: str | None = None,
        scopes: tuple[str, ...] = (),
    ) -> "AuthSettings":
        return cls(
            issuer=validate_service_url(
                issuer or os.environ.get("SEMATTICE_KEYCLOAK_ISSUER", DEFAULT_ISSUER),
                "Keycloak issuer",
            ),
            client_id=(client_id or os.environ.get("SEMATTICE_OIDC_CLIENT_ID", DEFAULT_CLIENT_ID)).strip(),
            semattice_base_url=validate_service_url(
                semattice_base_url or os.environ.get("SEMATTICE_BASE_URL", DEFAULT_SEMATTICE_BASE_URL),
                "Semattice 服务地址",
            ),
            scopes=tuple(dict.fromkeys(scope.strip() for scope in scopes if scope.strip())),
        )

    def __post_init__(self) -> None:
        if not self.client_id:
            raise AuthError("OIDC client ID 不能为空")


@dataclass
class CachedSession:
    version: int
    issuer: str
    client_id: str
    semattice_base_url: str
    company_id: str
    scopes: list[str]
    credential_account: str
    oact: str
    oact_expires_at: int

    @classmethod
    def from_json(cls, value: dict[str, Any]) -> "CachedSession":
        try:
            session = cls(
                version=int(value["version"]),
                issuer=str(value["issuer"]),
                client_id=str(value["client_id"]),
                semattice_base_url=str(value["semattice_base_url"]),
                company_id=str(value["company_id"]),
                scopes=[str(item) for item in value.get("scopes", [])],
                credential_account=str(value["credential_account"]),
                oact=str(value["oact"]),
                oact_expires_at=int(value["oact_expires_at"]),
            )
        except (KeyError, TypeError, ValueError) as exc:
            raise AuthError("本地登录缓存格式无效，请重新登录") from exc
        if session.version != SESSION_CACHE_VERSION or not all(
            [session.issuer, session.client_id, session.company_id, session.credential_account, session.oact]
        ):
            raise AuthError("本地登录缓存版本不兼容，请重新登录")
        return session


class SessionCache:
    def __init__(self, path: Path):
        self.path = path

    def load(self) -> CachedSession:
        try:
            info = self.path.lstat()
        except FileNotFoundError as exc:
            raise AuthError("尚未登录；请先执行 semattice login 或设置 SEMATTICE_TOKEN") from exc
        if stat.S_ISLNK(info.st_mode):
            raise AuthError("拒绝读取符号链接形式的登录缓存")
        if hasattr(os, "getuid") and info.st_uid != os.getuid():
            raise AuthError("登录缓存不属于当前用户")
        if stat.S_IMODE(info.st_mode) & 0o077:
            raise AuthError("登录缓存权限过宽；请改为 0600 后重试")
        descriptor: int | None = None
        try:
            descriptor = os.open(self.path, os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0))
            opened_info = os.fstat(descriptor)
            if opened_info.st_dev != info.st_dev or opened_info.st_ino != info.st_ino:
                raise AuthError("登录缓存在读取期间发生变化，请重试")
            with os.fdopen(descriptor, mode="r", encoding="utf-8") as handle:
                descriptor = None
                value = json.load(handle)
        except (OSError, json.JSONDecodeError) as exc:
            raise AuthError("无法读取本地登录缓存，请重新登录") from exc
        finally:
            if descriptor is not None:
                os.close(descriptor)
        if not isinstance(value, dict):
            raise AuthError("本地登录缓存格式无效，请重新登录")
        return CachedSession.from_json(value)

    def save(self, session: CachedSession) -> None:
        if self.path.is_symlink():
            raise AuthError("拒绝覆盖符号链接形式的登录缓存")
        parent = self.path.parent
        parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        try:
            parent_info = parent.stat()
            if hasattr(os, "getuid") and parent_info.st_uid != os.getuid():
                raise AuthError("登录缓存目录不属于当前用户")
            os.chmod(parent, 0o700)
        except OSError as exc:
            raise AuthError("无法保护登录缓存目录") from exc

        payload = json.dumps(asdict(session), ensure_ascii=False, separators=(",", ":"))
        temporary_path: Path | None = None
        try:
            with tempfile.NamedTemporaryFile(
                mode="w", encoding="utf-8", dir=parent, prefix=".credentials-", delete=False
            ) as handle:
                temporary_path = Path(handle.name)
                os.chmod(handle.name, 0o600)
                handle.write(payload)
                handle.flush()
                os.fsync(handle.fileno())
            os.replace(temporary_path, self.path)
            os.chmod(self.path, 0o600)
        except OSError as exc:
            if temporary_path is not None:
                temporary_path.unlink(missing_ok=True)
            raise AuthError("无法安全保存本地登录缓存") from exc

    def delete(self) -> None:
        if self.path.is_symlink():
            raise AuthError("拒绝删除符号链接形式的登录缓存")
        self.path.unlink(missing_ok=True)


class CredentialStore(Protocol):
    def save(self, account: str, secret: str) -> None: ...

    def load(self, account: str) -> str: ...

    def delete(self, account: str) -> None: ...


class MacOSKeychainStore:
    _ITEM_NOT_FOUND = -25300

    def __init__(self) -> None:
        try:
            self.security = ctypes.cdll.LoadLibrary(
                "/System/Library/Frameworks/Security.framework/Security"
            )
            self.core_foundation = ctypes.cdll.LoadLibrary(
                "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
            )
        except OSError as exc:
            raise AuthError("macOS Keychain 不可用") from exc
        self._configure_functions()

    def _configure_functions(self) -> None:
        u32 = ctypes.c_uint32
        pointer = ctypes.c_void_p
        self.security.SecKeychainFindGenericPassword.argtypes = [
            pointer,
            u32,
            ctypes.c_char_p,
            u32,
            ctypes.c_char_p,
            ctypes.POINTER(u32),
            ctypes.POINTER(pointer),
            ctypes.POINTER(pointer),
        ]
        self.security.SecKeychainFindGenericPassword.restype = ctypes.c_int32
        self.security.SecKeychainAddGenericPassword.argtypes = [
            pointer,
            u32,
            ctypes.c_char_p,
            u32,
            ctypes.c_char_p,
            u32,
            pointer,
            ctypes.POINTER(pointer),
        ]
        self.security.SecKeychainAddGenericPassword.restype = ctypes.c_int32
        self.security.SecKeychainItemModifyAttributesAndData.argtypes = [
            pointer,
            pointer,
            u32,
            pointer,
        ]
        self.security.SecKeychainItemModifyAttributesAndData.restype = ctypes.c_int32
        self.security.SecKeychainItemDelete.argtypes = [pointer]
        self.security.SecKeychainItemDelete.restype = ctypes.c_int32
        self.security.SecKeychainItemFreeContent.argtypes = [pointer, pointer]
        self.security.SecKeychainItemFreeContent.restype = ctypes.c_int32
        self.core_foundation.CFRelease.argtypes = [pointer]
        self.core_foundation.CFRelease.restype = None

    @staticmethod
    def _bytes(value: str) -> bytes:
        return value.encode("utf-8")

    def _find(self, account: str, include_secret: bool) -> tuple[int, ctypes.c_void_p, str | None]:
        service = self._bytes(KEYCHAIN_SERVICE)
        account_bytes = self._bytes(account)
        item = ctypes.c_void_p()
        secret_length = ctypes.c_uint32()
        secret_pointer = ctypes.c_void_p()
        status_code = self.security.SecKeychainFindGenericPassword(
            None,
            len(service),
            service,
            len(account_bytes),
            account_bytes,
            ctypes.byref(secret_length) if include_secret else None,
            ctypes.byref(secret_pointer) if include_secret else None,
            ctypes.byref(item),
        )
        secret_value = None
        if status_code == 0 and include_secret:
            try:
                secret_value = ctypes.string_at(secret_pointer, secret_length.value).decode("utf-8")
            finally:
                self.security.SecKeychainItemFreeContent(None, secret_pointer)
        return status_code, item, secret_value

    def save(self, account: str, secret: str) -> None:
        secret_bytes = self._bytes(secret)
        secret_buffer = ctypes.create_string_buffer(secret_bytes)
        status_code, item, _ = self._find(account, include_secret=False)
        try:
            if status_code == 0:
                result = self.security.SecKeychainItemModifyAttributesAndData(
                    item, None, len(secret_bytes), ctypes.cast(secret_buffer, ctypes.c_void_p)
                )
            elif status_code == self._ITEM_NOT_FOUND:
                service = self._bytes(KEYCHAIN_SERVICE)
                account_bytes = self._bytes(account)
                result = self.security.SecKeychainAddGenericPassword(
                    None,
                    len(service),
                    service,
                    len(account_bytes),
                    account_bytes,
                    len(secret_bytes),
                    ctypes.cast(secret_buffer, ctypes.c_void_p),
                    None,
                )
            else:
                raise AuthError(f"无法访问 macOS Keychain（状态 {status_code}）")
        finally:
            if item:
                self.core_foundation.CFRelease(item)
        if result != 0:
            raise AuthError(f"无法写入 macOS Keychain（状态 {result}）")

    def load(self, account: str) -> str:
        status_code, item, secret_value = self._find(account, include_secret=True)
        if item:
            self.core_foundation.CFRelease(item)
        if status_code == self._ITEM_NOT_FOUND:
            raise AuthError("系统凭据库中没有可续期会话，请重新登录")
        if status_code != 0 or not secret_value:
            raise AuthError(f"无法读取 macOS Keychain（状态 {status_code}）")
        return secret_value

    def delete(self, account: str) -> None:
        status_code, item, _ = self._find(account, include_secret=False)
        if status_code == self._ITEM_NOT_FOUND:
            return
        if status_code != 0:
            raise AuthError(f"无法访问 macOS Keychain（状态 {status_code}）")
        try:
            result = self.security.SecKeychainItemDelete(item)
        finally:
            if item:
                self.core_foundation.CFRelease(item)
        if result != 0:
            raise AuthError(f"无法删除 macOS Keychain 凭据（状态 {result}）")


class SecretToolStore:
    def __init__(self) -> None:
        self.command = shutil.which("secret-tool")
        if not self.command:
            raise AuthError("未找到安全凭据库；请安装 secret-tool 或使用短期 SEMATTICE_TOKEN")

    def _run(self, arguments: list[str], *, secret_input: str | None = None) -> subprocess.CompletedProcess[bytes]:
        return subprocess.run(
            [self.command, *arguments],
            input=secret_input.encode("utf-8") if secret_input is not None else None,
            stdout=subprocess.PIPE,
            stderr=subprocess.DEVNULL,
            check=False,
        )

    def save(self, account: str, secret: str) -> None:
        result = self._run(
            ["store", "--label", "CloudCC Semattice CLI", "service", KEYCHAIN_SERVICE, "account", account],
            secret_input=secret,
        )
        if result.returncode != 0:
            raise AuthError("无法写入系统 Secret Service")

    def load(self, account: str) -> str:
        result = self._run(["lookup", "service", KEYCHAIN_SERVICE, "account", account])
        secret_value = result.stdout.decode("utf-8").strip()
        if result.returncode != 0 or not secret_value:
            raise AuthError("系统凭据库中没有可续期会话，请重新登录")
        return secret_value

    def delete(self, account: str) -> None:
        result = self._run(["clear", "service", KEYCHAIN_SERVICE, "account", account])
        if result.returncode not in {0, 1}:
            raise AuthError("无法删除系统 Secret Service 凭据")


def system_credential_store() -> CredentialStore:
    current = platform.system()
    if current == "Darwin":
        return MacOSKeychainStore()
    if current == "Linux":
        return SecretToolStore()
    raise AuthError("当前系统没有受支持的安全凭据库；请使用短期 SEMATTICE_TOKEN")


class JSONHTTPClient:
    def __init__(self, timeout: float = 30.0):
        self.timeout = timeout

    def _request(
        self,
        method: str,
        url: str,
        *,
        body: bytes | None = None,
        content_type: str | None = None,
        bearer: str | None = None,
    ) -> Any:
        headers = {"Accept": "application/json"}
        if content_type:
            headers["Content-Type"] = content_type
        if bearer:
            headers["Authorization"] = f"Bearer {bearer}"
        request = urllib.request.Request(url, data=body, method=method, headers=headers)
        try:
            with open_without_redirect(request, timeout=self.timeout) as response:
                raw_response = response.read()
        except urllib.error.HTTPError as exc:
            try:
                raw_response = exc.read()
            except OSError:
                raw_response = b""
            error_code = ""
            try:
                payload = json.loads(raw_response)
                candidate = payload.get("error") if isinstance(payload, dict) else None
                if isinstance(candidate, str) and SAFE_ERROR_CODE.fullmatch(candidate):
                    error_code = candidate
            except (json.JSONDecodeError, AttributeError):
                pass
            suffix = f"（{error_code}）" if error_code else ""
            raise AuthError(f"认证服务返回 HTTP {exc.code}{suffix}") from exc
        except urllib.error.URLError as exc:
            raise AuthError(f"无法连接认证服务：{exc.reason}") from exc
        try:
            return json.loads(raw_response)
        except json.JSONDecodeError as exc:
            raise AuthError("认证服务返回了非 JSON 响应") from exc

    def get_json(self, url: str, *, bearer: str | None = None) -> Any:
        return self._request("GET", url, bearer=bearer)

    def post_form(self, url: str, values: dict[str, str]) -> Any:
        body = urllib.parse.urlencode(values).encode("utf-8")
        return self._request(
            "POST", url, body=body, content_type="application/x-www-form-urlencoded"
        )

    def post_json(self, url: str, value: dict[str, Any], *, bearer: str) -> Any:
        body = json.dumps(value, separators=(",", ":")).encode("utf-8")
        return self._request("POST", url, body=body, content_type="application/json", bearer=bearer)


def credential_account(issuer: str, client_id: str) -> str:
    return hashlib.sha256(f"{issuer}\0{client_id}".encode("utf-8")).hexdigest()


def authorization_url(
    authorization_endpoint: str,
    *,
    client_id: str,
    redirect_uri: str,
    state_value: str,
    code_challenge: str,
    organization: str | None = None,
) -> str:
    organization_scope = f"organization:{organization}" if organization else "organization"
    query = urllib.parse.urlencode(
        {
            "response_type": "code",
            "client_id": client_id,
            "redirect_uri": redirect_uri,
            "scope": f"openid offline_access {organization_scope}",
            "state": state_value,
            "code_challenge": code_challenge,
            "code_challenge_method": "S256",
        }
    )
    return f"{authorization_endpoint}?{query}"


def parse_callback_query(query: str, expected_state: str) -> str:
    values = urllib.parse.parse_qs(query, keep_blank_values=True)
    if values.get("state", [""])[0] != expected_state:
        raise AuthError("Keycloak 回调 state 不匹配，登录已拒绝")
    error_value = values.get("error", [""])[0]
    if error_value:
        raise AuthError(f"Keycloak 登录未完成：{error_value}")
    code_value = values.get("code", [""])[0]
    if not code_value:
        raise AuthError("Keycloak 回调缺少 authorization code")
    return code_value


def callback_page(succeeded: bool) -> bytes:
    page_class = "success" if succeeded else "failure"
    aria_role = "status" if succeeded else "alert"
    status_label = "AUTHORIZATION RECEIVED" if succeeded else "AUTHORIZATION INTERRUPTED"
    headline = "身份验证完成" if succeeded else "身份验证未完成"
    detail = (
        "正在返回终端完成安全连接。你现在可以关闭此窗口。"
        if succeeded
        else "请返回终端查看原因，然后重新发起登录。"
    )
    symbol = (
        '<path d="m7.5 12.5 3 3 6-7"/>'
        if succeeded
        else '<path d="M9 9l6 6M15 9l-6 6"/>'
    )
    return f"""<!doctype html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <meta name="color-scheme" content="dark">
  <title>{headline} · CloudCC Semattice</title>
  <style>
    :root {{
      color-scheme: dark;
      --ink: #0b0d0c;
      --paper: #ece9df;
      --muted: #9c9f97;
      --line: rgba(236, 233, 223, .18);
      --acid: #c8ff3d;
      --cyan: #4ee8d3;
      --danger: #ff8d84;
      --serif: "Iowan Old Style", "Baskerville", Georgia, serif;
      --mono: "SFMono-Regular", "Cascadia Code", "Liberation Mono", Menlo, monospace;
    }}
    * {{ box-sizing: border-box; }}
    body {{
      min-height: 100vh;
      margin: 0;
      display: grid;
      place-items: center;
      padding: 28px;
      overflow: hidden;
      color: var(--paper);
      background:
        radial-gradient(circle at 82% 12%, rgba(78, 232, 211, .11), transparent 32%),
        radial-gradient(circle at 12% 88%, rgba(200, 255, 61, .08), transparent 28%),
        linear-gradient(rgba(255, 255, 255, .026) 1px, transparent 1px),
        linear-gradient(90deg, rgba(255, 255, 255, .026) 1px, transparent 1px),
        var(--ink);
      background-size: auto, auto, 42px 42px, 42px 42px, auto;
      font-family: var(--mono);
    }}
    body::before {{
      content: "";
      position: fixed;
      width: 360px;
      height: 360px;
      border: 1px solid var(--line);
      transform: rotate(45deg);
      opacity: .36;
      pointer-events: none;
    }}
    .card {{
      position: relative;
      width: min(560px, 100%);
      border: 1px solid var(--line);
      background: rgba(11, 13, 12, .86);
      box-shadow: 24px 24px 0 rgba(200, 255, 61, .035);
      backdrop-filter: blur(18px);
    }}
    .card::before {{
      content: "";
      position: absolute;
      inset: -1px auto auto -1px;
      width: 92px;
      height: 3px;
      background: var(--accent);
    }}
    .success {{ --accent: var(--acid); }}
    .failure {{ --accent: var(--danger); }}
    .header {{
      min-height: 72px;
      padding: 0 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      border-bottom: 1px solid var(--line);
    }}
    .brand {{ display: flex; align-items: center; gap: 12px; }}
    .brand-mark {{
      width: 30px;
      height: 30px;
      display: grid;
      grid-template: 1fr 1fr / 1fr 1fr;
      border: 1px solid var(--paper);
      transform: rotate(45deg);
    }}
    .brand-mark i {{ border: 1px solid rgba(236, 233, 223, .25); }}
    .brand-mark i:first-child {{ background: var(--accent); }}
    .brand-name {{ display: grid; gap: 3px; }}
    .brand-name strong {{ font-size: 10px; letter-spacing: .16em; }}
    .brand-name span {{ color: var(--muted); font: italic 18px/1 var(--serif); }}
    .secure {{
      display: flex;
      align-items: center;
      gap: 7px;
      color: var(--muted);
      font-size: 8px;
      letter-spacing: .12em;
    }}
    .secure::before {{
      content: "";
      width: 6px;
      height: 6px;
      border-radius: 50%;
      background: var(--cyan);
      box-shadow: 0 0 12px var(--cyan);
    }}
    .content {{ padding: 52px 48px 46px; }}
    .status-mark {{
      width: 76px;
      height: 76px;
      display: grid;
      place-items: center;
      border: 1px solid var(--accent);
      color: var(--ink);
      background: var(--accent);
      transform: rotate(45deg);
      box-shadow: 0 0 54px color-mix(in srgb, var(--accent) 17%, transparent);
    }}
    .status-mark svg {{
      width: 34px;
      height: 34px;
      transform: rotate(-45deg);
      fill: none;
      stroke: currentColor;
      stroke-width: 1.8;
      stroke-linecap: round;
      stroke-linejoin: round;
    }}
    .eyebrow {{
      margin: 42px 0 12px;
      color: var(--accent);
      font-size: 9px;
      font-weight: 700;
      letter-spacing: .17em;
    }}
    h1 {{
      margin: 0;
      font: italic clamp(42px, 9vw, 62px)/.95 var(--serif);
      letter-spacing: -.035em;
    }}
    .detail {{
      max-width: 410px;
      margin: 24px 0 0;
      color: #c9c8c1;
      font: 13px/1.9 -apple-system, BlinkMacSystemFont, "PingFang SC", "Microsoft YaHei", sans-serif;
    }}
    .footer {{
      min-height: 58px;
      padding: 0 24px;
      display: flex;
      align-items: center;
      justify-content: space-between;
      gap: 18px;
      border-top: 1px solid var(--line);
      color: var(--muted);
      font-size: 8px;
      letter-spacing: .1em;
    }}
    .footer b {{ color: var(--paper); font-weight: 500; }}
    @media (max-width: 560px) {{
      body {{ padding: 16px; }}
      .header, .footer {{ padding-inline: 18px; }}
      .secure {{ display: none; }}
      .content {{ padding: 42px 24px 38px; }}
      .status-mark {{ width: 64px; height: 64px; }}
      .detail {{ font-size: 12px; }}
    }}
    @media (prefers-reduced-motion: no-preference) {{
      .card {{ animation: arrive .45s cubic-bezier(.2, .75, .25, 1) both; }}
      .status-mark {{ animation: settle .55s .12s cubic-bezier(.2, .75, .25, 1) both; }}
      @keyframes arrive {{
        from {{ opacity: 0; transform: translateY(14px); }}
        to {{ opacity: 1; transform: none; }}
      }}
      @keyframes settle {{
        from {{ opacity: 0; transform: rotate(45deg) scale(.72); }}
        to {{ opacity: 1; transform: rotate(45deg) scale(1); }}
      }}
    }}
  </style>
</head>
<body>
  <main class="card {page_class}" aria-labelledby="callback-title">
    <header class="header">
      <div class="brand" aria-label="CloudCC Semattice">
        <span class="brand-mark" aria-hidden="true"><i></i><i></i><i></i><i></i></span>
        <span class="brand-name"><strong>CLOUDCC</strong><span>Semattice</span></span>
      </div>
      <span class="secure">SECURE LOOPBACK</span>
    </header>
    <section class="content" role="{aria_role}">
      <div class="status-mark" aria-hidden="true">
        <svg viewBox="0 0 24 24">{symbol}</svg>
      </div>
      <p class="eyebrow">{status_label}</p>
      <h1 id="callback-title">{headline}</h1>
      <p class="detail">{detail}</p>
    </section>
    <footer class="footer">
      <span><b>LOCAL CALLBACK</b> · 127.0.0.1</span>
      <span>NO CREDENTIALS SHOWN</span>
    </footer>
  </main>
</body>
</html>
""".encode("utf-8")


def receive_authorization_code(
    authorization_endpoint: str,
    *,
    client_id: str,
    state_value: str,
    code_challenge: str,
    timeout: float,
    no_browser: bool,
    organization: str | None = None,
) -> tuple[str, str]:
    result: dict[str, str] = {}

    class CallbackHandler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:  # noqa: N802 - required by BaseHTTPRequestHandler
            parsed = urllib.parse.urlparse(self.path)
            try:
                if parsed.path not in {"", "/"}:
                    raise AuthError("无效的回调路径")
                result["code"] = parse_callback_query(parsed.query, state_value)
                body = callback_page(True)
                status_code = 200
            except AuthError as exc:
                result["error"] = str(exc)
                body = callback_page(False)
                status_code = 400
            self.send_response(status_code)
            self.send_header("Content-Type", "text/html; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header(
                "Content-Security-Policy",
                "default-src 'none'; style-src 'unsafe-inline'; "
                "base-uri 'none'; form-action 'none'; frame-ancestors 'none'",
            )
            self.send_header("Referrer-Policy", "no-referrer")
            self.send_header("X-Content-Type-Options", "nosniff")
            self.send_header("X-Frame-Options", "DENY")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)

        def log_message(self, _format: str, *_args: Any) -> None:
            return

    server = HTTPServer(("127.0.0.1", 0), CallbackHandler)
    server.timeout = timeout
    redirect_uri = f"http://127.0.0.1:{server.server_port}"
    login_url = authorization_url(
        authorization_endpoint,
        client_id=client_id,
        redirect_uri=redirect_uri,
        state_value=state_value,
        code_challenge=code_challenge,
        organization=organization,
    )
    print("请在浏览器中完成 Keycloak 登录。", file=sys.stderr)
    if no_browser or not webbrowser.open(login_url, new=1, autoraise=True):
        print(f"请手动打开：{login_url}", file=sys.stderr)
    try:
        server.handle_request()
    finally:
        server.server_close()
    if result.get("error"):
        raise AuthError(result["error"])
    if not result.get("code"):
        raise AuthError("等待 Keycloak 登录回调超时")
    return result["code"], redirect_uri


class AuthManager:
    def __init__(
        self,
        cache_path: Path | None = None,
        *,
        store: CredentialStore | None = None,
        http: JSONHTTPClient | None = None,
        callback_receiver: Callable[..., tuple[str, str]] = receive_authorization_code,
        now: Callable[[], float] = time.time,
    ):
        self.cache = SessionCache(cache_path or default_credentials_file())
        self._store = store
        self.http = http or JSONHTTPClient()
        self.callback_receiver = callback_receiver
        self.now = now

    @property
    def store(self) -> CredentialStore:
        if self._store is None:
            self._store = system_credential_store()
        return self._store

    def _discover(self, issuer: str) -> dict[str, Any]:
        value = self.http.get_json(f"{issuer}/.well-known/openid-configuration")
        if not isinstance(value, dict) or value.get("issuer") != issuer:
            raise AuthError("OIDC discovery issuer 与配置不一致")
        for key in ("authorization_endpoint", "token_endpoint"):
            endpoint = value.get(key)
            if not isinstance(endpoint, str):
                raise AuthError(f"OIDC discovery 缺少 {key}")
            validate_service_url(endpoint, key)
        return value

    @staticmethod
    def _token_payload(value: Any, *, require_refresh: bool) -> tuple[str, str | None]:
        if not isinstance(value, dict):
            raise AuthError("Keycloak Token 响应格式无效")
        access_token = value.get("access_token")
        refresh_token = value.get("refresh_token")
        if not isinstance(access_token, str) or not access_token:
            raise AuthError("Keycloak Token 响应缺少 access_token")
        if require_refresh and (not isinstance(refresh_token, str) or not refresh_token):
            raise AuthError("Keycloak 未返回 refresh_token；请确认允许 offline_access")
        return access_token, refresh_token if isinstance(refresh_token, str) and refresh_token else None

    def _mint_oact(
        self,
        *,
        semattice_base_url: str,
        access_token: str,
        scopes: tuple[str, ...],
    ) -> tuple[str, int, str]:
        value = self.http.post_json(
            f"{semattice_base_url}{DEFAULT_TOKEN_PATH}",
            {"requested_scopes": list(scopes)},
            bearer=access_token,
        )
        if not isinstance(value, dict):
            raise AuthError("Semattice OACT 响应格式无效")
        oact = value.get("access_token")
        if not isinstance(oact, str) or not oact:
            raise AuthError("Semattice OACT 响应缺少 access_token")
        company_id = value.get("company_id")
        if not isinstance(company_id, str) or not company_id:
            raise AuthError("Semattice OACT 响应缺少 company_id")
        token_type = str(value.get("token_type", "Bearer"))
        if token_type.lower() != "bearer":
            raise AuthError("Semattice 返回了不受支持的 token_type")
        try:
            expires_at = int(self.now()) + int(value.get("expires_in", 600))
        except (TypeError, ValueError) as exc:
            raise AuthError("Semattice OACT expires_in 无效") from exc
        embedded_exp = jwt_exp(oact)
        if embedded_exp is not None:
            expires_at = min(expires_at, embedded_exp)
        if expires_at <= int(self.now()):
            raise AuthError("Semattice 返回了已过期的 OACT")
        return oact, expires_at, company_id

    def login(
        self,
        settings: AuthSettings,
        *,
        requested_company: str | None = None,
        no_browser: bool = False,
        timeout: float = 180.0,
    ) -> CachedSession:
        discovery = self._discover(settings.issuer)
        verifier, challenge = generate_pkce()
        state_value = secrets.token_urlsafe(32)
        code_value, redirect_uri = self.callback_receiver(
            discovery["authorization_endpoint"],
            client_id=settings.client_id,
            state_value=state_value,
            code_challenge=challenge,
            timeout=timeout,
            no_browser=no_browser,
            organization=requested_company,
        )
        token_response = self.http.post_form(
            discovery["token_endpoint"],
            {
                "grant_type": "authorization_code",
                "client_id": settings.client_id,
                "code": code_value,
                "redirect_uri": redirect_uri,
                "code_verifier": verifier,
            },
        )
        access_token, refresh_token = self._token_payload(token_response, require_refresh=True)
        oact, expires_at, company_id = self._mint_oact(
            semattice_base_url=settings.semattice_base_url,
            access_token=access_token,
            scopes=settings.scopes,
        )
        account = credential_account(settings.issuer, settings.client_id)
        session = CachedSession(
            version=SESSION_CACHE_VERSION,
            issuer=settings.issuer,
            client_id=settings.client_id,
            semattice_base_url=settings.semattice_base_url,
            company_id=company_id,
            scopes=list(settings.scopes),
            credential_account=account,
            oact=oact,
            oact_expires_at=expires_at,
        )
        self.store.save(account, refresh_token or "")
        try:
            self.cache.save(session)
        except Exception:
            self.store.delete(account)
            raise
        return session

    def renew(self, session: CachedSession) -> CachedSession:
        discovery = self._discover(session.issuer)
        refresh_token = self.store.load(session.credential_account)
        token_response = self.http.post_form(
            discovery["token_endpoint"],
            {
                "grant_type": "refresh_token",
                "client_id": session.client_id,
                "refresh_token": refresh_token,
            },
        )
        access_token, rotated_refresh_token = self._token_payload(
            token_response, require_refresh=False
        )
        if rotated_refresh_token:
            self.store.save(session.credential_account, rotated_refresh_token)
        oact, expires_at, company_id = self._mint_oact(
            semattice_base_url=session.semattice_base_url,
            access_token=access_token,
            scopes=tuple(session.scopes),
        )
        if company_id != session.company_id:
            raise AuthError("Keycloak 刷新后的组织与当前会话不一致，请重新登录")
        session.oact = oact
        session.oact_expires_at = expires_at
        self.cache.save(session)
        return session

    def get_session(self, *, force_refresh: bool = False) -> CachedSession:
        session = self.cache.load()
        if force_refresh or session.oact_expires_at <= int(self.now()) + TOKEN_SKEW_SECONDS:
            session = self.renew(session)
        return session

    def status(self) -> dict[str, Any]:
        session = self.cache.load()
        remaining = max(0, session.oact_expires_at - int(self.now()))
        return {
            "status": "authenticated" if remaining > 0 else "expired",
            "issuer": session.issuer,
            "client_id": session.client_id,
            "company_id": session.company_id,
            "semattice_base_url": session.semattice_base_url,
            "oact_expires_at": session.oact_expires_at,
            "oact_remaining_seconds": remaining,
            "scope_count": len(session.scopes),
            "scopes": list(session.scopes),
        }

    def logout(self, fallback_settings: AuthSettings) -> None:
        try:
            session = self.cache.load()
            account = session.credential_account
        except AuthError:
            account = credential_account(fallback_settings.issuer, fallback_settings.client_id)
        self.store.delete(account)
        self.cache.delete()
