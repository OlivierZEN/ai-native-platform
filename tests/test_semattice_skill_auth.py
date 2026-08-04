from __future__ import annotations

import io
import ctypes
import json
import os
import stat
import sys
import tempfile
import threading
import unittest
import urllib.error
import urllib.parse
import urllib.request
from types import SimpleNamespace
from argparse import Namespace
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path
from unittest import mock


SCRIPT_DIR = Path(__file__).resolve().parents[1] / "skill" / "cloudcc-semattice" / "scripts"
sys.path.insert(0, str(SCRIPT_DIR))

import semattice_api  # noqa: E402
from semattice_auth import (  # noqa: E402
    AuthError,
    AuthManager,
    AuthSettings,
    CachedSession,
    DEFAULT_CAPABILITY_SCOPES,
    JSONHTTPClient,
    SESSION_CACHE_VERSION,
    SessionCache,
    WindowsCredentialStore,
    _WindowsCredential,
    authorization_url,
    default_credentials_file,
    generate_pkce,
    parse_callback_query,
    receive_authorization_code,
    system_credential_store,
)


class FakeCredentialStore:
    def __init__(self) -> None:
        self.values: dict[str, str] = {}

    def save(self, account: str, secret: str) -> None:
        self.values[account] = secret

    def load(self, account: str) -> str:
        try:
            return self.values[account]
        except KeyError as exc:
            raise AuthError("missing credential") from exc

    def delete(self, account: str) -> None:
        self.values.pop(account, None)


class FakeAuthHTTP:
    def __init__(self) -> None:
        self.forms: list[dict[str, str]] = []
        self.mints: list[dict[str, object]] = []
        self.mint_count = 0

    def get_json(self, url: str, *, bearer: str | None = None) -> object:
        if url.endswith("/.well-known/openid-configuration"):
            return {
                "issuer": "https://sso.example.test/realms/example",
                "authorization_endpoint": "https://sso.example.test/authorize",
                "token_endpoint": "https://sso.example.test/token",
            }
        raise AssertionError(url)

    def post_form(self, url: str, values: dict[str, str]) -> object:
        self.forms.append(values)
        if values["grant_type"] == "authorization_code":
            return {"access_token": "keycloak-access-1", "refresh_token": "keycloak-refresh-1"}
        if values["grant_type"] == "refresh_token":
            return {"access_token": "keycloak-access-2", "refresh_token": "keycloak-refresh-2"}
        raise AssertionError(values)

    def post_json(self, url: str, value: dict[str, object], *, bearer: str) -> object:
        self.mints.append({"url": url, "value": value, "bearer": bearer})
        self.mint_count += 1
        return {
            "access_token": f"short-oact-{self.mint_count}",
            "token_type": "Bearer",
            "expires_in": 600,
            "company_id": "org-example",
        }


class FakeResponse:
    def __init__(self, status: int, body: dict[str, object]) -> None:
        self.status = status
        self.body = json.dumps(body).encode("utf-8")

    def __enter__(self) -> "FakeResponse":
        return self

    def __exit__(self, *_args: object) -> None:
        return None

    def read(self) -> bytes:
        return self.body


class ResettingErrorBody:
    def read(self, _size: int = -1) -> bytes:
        raise ConnectionResetError("simulated reset")

    def close(self) -> None:
        return


class SematticeAuthenticationTests(unittest.TestCase):
    def setUp(self) -> None:
        self.temp_dir = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp_dir.cleanup)
        self.cache_path = Path(self.temp_dir.name) / "auth" / "credentials.json"
        self.store = FakeCredentialStore()
        self.http = FakeAuthHTTP()
        self.now_value = [1_000.0]
        self.callback_values: dict[str, object] = {}

        def callback_receiver(endpoint: str, **values: object) -> tuple[str, str]:
            self.callback_values = {"endpoint": endpoint, **values}
            return "authorization-code", "http://127.0.0.1:32123"

        self.manager = AuthManager(
            self.cache_path,
            store=self.store,
            http=self.http,
            callback_receiver=callback_receiver,
            now=lambda: self.now_value[0],
        )
        self.settings = AuthSettings.from_values(
            issuer="https://sso.example.test/realms/example",
            client_id="semattice-cli",
            semattice_base_url="https://semattice.example.test",
            scopes=DEFAULT_CAPABILITY_SCOPES,
        )

    def test_pkce_authorization_url_uses_s256_and_loopback(self) -> None:
        verifier, challenge = generate_pkce()
        self.assertGreaterEqual(len(verifier), 43)
        url = authorization_url(
            "https://sso.example.test/authorize",
            client_id="semattice-cli",
            redirect_uri="http://127.0.0.1:32123",
            state_value="random-state",
            code_challenge=challenge,
        )
        query = urllib.parse.parse_qs(urllib.parse.urlparse(url).query)
        self.assertEqual(query["response_type"], ["code"])
        self.assertEqual(query["code_challenge_method"], ["S256"])
        self.assertEqual(query["state"], ["random-state"])
        self.assertEqual(query["redirect_uri"], ["http://127.0.0.1:32123"])
        self.assertEqual(query["scope"], ["openid offline_access organization"])
        self.assertNotIn(verifier, url)
        selected = authorization_url(
            "https://sso.example.test/authorize",
            client_id="semattice-cli",
            redirect_uri="http://127.0.0.1:32123",
            state_value="random-state",
            code_challenge=challenge,
            organization="org2sva14i4udjmi2t4s",
        )
        selected_query = urllib.parse.parse_qs(urllib.parse.urlparse(selected).query)
        self.assertEqual(
            selected_query["scope"],
            ["openid offline_access organization:org2sva14i4udjmi2t4s"],
        )

    def test_service_urls_reject_embedded_credentials_and_fragments(self) -> None:
        with self.assertRaisesRegex(AuthError, "用户凭据"):
            AuthSettings.from_values(issuer="https://user:password@sso.example.test/realms/example")
        with self.assertRaisesRegex(AuthError, "query 或 fragment"):
            AuthSettings.from_values(semattice_base_url="https://semattice.example.test/#token")

    def test_login_defaults_to_all_published_capability_scopes(self) -> None:
        args = semattice_api.command_parser().parse_args(["login"])
        self.assertEqual(args.scope, list(DEFAULT_CAPABILITY_SCOPES))
        self.assertEqual(len(args.scope), 26)
        self.assertEqual(len(set(args.scope)), 26)
        self.assertIn("runtime.record.create", args.scope)
        self.assertIn("authorization.manage", args.scope)
        self.assertNotIn("tenant.provision", args.scope)

    def test_default_scopes_match_human_skill_catalog_capabilities(self) -> None:
        catalog_path = SCRIPT_DIR.parent / "references" / "api-catalog.md"
        catalog_scopes: set[str] = set()
        capability_count = 0
        for line in catalog_path.read_text(encoding="utf-8").splitlines():
            columns = [column.strip() for column in line.split("|")]
            if len(columns) == 8 and columns[1].startswith("`") and columns[2] in {"v1", "v2"}:
                capability_count += 1
                catalog_scopes.add(columns[4].strip("`"))
        self.assertEqual(capability_count, 57)
        self.assertEqual(
            catalog_scopes,
            set(DEFAULT_CAPABILITY_SCOPES) | {"identity.principal.sync"},
        )

    def test_bearer_redirect_is_rejected_and_error_description_is_redacted(self) -> None:
        destination_requests: list[str | None] = []

        class DestinationHandler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                destination_requests.append(self.headers.get("Authorization"))
                self.send_response(200)
                self.end_headers()
                self.wfile.write(b"{}")

            def log_message(self, _format: str, *_args: object) -> None:
                return

        destination = HTTPServer(("127.0.0.1", 0), DestinationHandler)

        class RedirectHandler(BaseHTTPRequestHandler):
            def do_GET(self) -> None:  # noqa: N802
                self.send_response(302)
                self.send_header("Location", f"http://127.0.0.1:{destination.server_port}/stolen")
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(
                    b'{"error":"invalid_request","error_description":"refresh-secret-must-not-print"}'
                )

            def do_POST(self) -> None:  # noqa: N802
                self.do_GET()

            def log_message(self, _format: str, *_args: object) -> None:
                return

        redirect = HTTPServer(("127.0.0.1", 0), RedirectHandler)
        destination_thread = threading.Thread(target=destination.serve_forever, daemon=True)
        redirect_thread = threading.Thread(target=redirect.serve_forever, daemon=True)
        destination_thread.start()
        redirect_thread.start()
        try:
            with self.assertRaises(AuthError) as raised:
                JSONHTTPClient(timeout=2).get_json(
                    f"http://127.0.0.1:{redirect.server_port}/start", bearer="keycloak-secret"
                )
            status_code, _payload = semattice_api.invoke_once(
                f"http://127.0.0.1:{redirect.server_port}/capability",
                {"request_id": "req-redirect", "input": {}},
                "short-oact-secret",
                2,
            )
        finally:
            redirect.shutdown()
            destination.shutdown()
            redirect.server_close()
            destination.server_close()
        message = str(raised.exception)
        self.assertIn("HTTP 302", message)
        self.assertIn("invalid_request", message)
        self.assertNotIn("refresh-secret", message)
        self.assertEqual(status_code, 302)
        self.assertEqual(destination_requests, [])

    def test_callback_rejects_state_mismatch_and_missing_code(self) -> None:
        with self.assertRaisesRegex(AuthError, "state"):
            parse_callback_query("state=wrong&code=secret", "expected")
        with self.assertRaisesRegex(AuthError, "authorization code"):
            parse_callback_query("state=expected", "expected")
        self.assertEqual(parse_callback_query("state=expected&code=ok", "expected"), "ok")

    def test_loopback_receiver_accepts_only_the_bound_state(self) -> None:
        callback_errors: list[BaseException] = []
        callback_response: dict[str, object] = {}
        callback_done = threading.Event()

        def browser_open(login_url: str, **_values: object) -> bool:
            query = urllib.parse.parse_qs(urllib.parse.urlparse(login_url).query)
            callback_url = (
                f"{query['redirect_uri'][0]}?"
                + urllib.parse.urlencode({"state": query["state"][0], "code": "loopback-code"})
            )

            def send_callback() -> None:
                try:
                    with urllib.request.urlopen(callback_url, timeout=2) as response:
                        callback_response["status"] = response.status
                        callback_response["content_type"] = response.headers["Content-Type"]
                        callback_response["cache_control"] = response.headers["Cache-Control"]
                        callback_response["csp"] = response.headers["Content-Security-Policy"]
                        callback_response["body"] = response.read().decode("utf-8")
                except BaseException as exc:  # test thread must report every failure
                    callback_errors.append(exc)
                finally:
                    callback_done.set()

            threading.Thread(target=send_callback, daemon=True).start()
            return True

        with mock.patch("semattice_auth.webbrowser.open", side_effect=browser_open):
            code_value, redirect_uri = receive_authorization_code(
                "https://sso.example.test/authorize",
                client_id="semattice-cli",
                state_value="expected-state",
                code_challenge="challenge",
                timeout=2,
                no_browser=False,
            )
        self.assertTrue(callback_done.wait(2))
        self.assertEqual(callback_errors, [])
        self.assertEqual(code_value, "loopback-code")
        parsed = urllib.parse.urlparse(redirect_uri)
        self.assertEqual(parsed.hostname, "127.0.0.1")
        self.assertEqual(parsed.path, "")
        self.assertEqual(callback_response["status"], 200)
        self.assertEqual(callback_response["content_type"], "text/html; charset=utf-8")
        self.assertEqual(callback_response["cache_control"], "no-store")
        self.assertIn("default-src 'none'", str(callback_response["csp"]))
        self.assertIn("CloudCC Semattice", str(callback_response["body"]))
        self.assertIn("身份验证完成", str(callback_response["body"]))
        self.assertNotIn("loopback-code", str(callback_response["body"]))

    def test_loopback_receiver_uses_safe_failure_page(self) -> None:
        callback_response: dict[str, object] = {}
        callback_done = threading.Event()

        def browser_open(login_url: str, **_values: object) -> bool:
            query = urllib.parse.parse_qs(urllib.parse.urlparse(login_url).query)
            callback_url = (
                f"{query['redirect_uri'][0]}?"
                + urllib.parse.urlencode({"state": "wrong-state", "code": "secret-code"})
            )

            def send_callback() -> None:
                try:
                    urllib.request.urlopen(callback_url, timeout=2)
                except urllib.error.HTTPError as exc:
                    callback_response["status"] = exc.code
                    callback_response["body"] = exc.read().decode("utf-8")
                finally:
                    callback_done.set()

            threading.Thread(target=send_callback, daemon=True).start()
            return True

        with mock.patch("semattice_auth.webbrowser.open", side_effect=browser_open):
            with self.assertRaisesRegex(AuthError, "state"):
                receive_authorization_code(
                    "https://sso.example.test/authorize",
                    client_id="semattice-cli",
                    state_value="expected-state",
                    code_challenge="challenge",
                    timeout=2,
                    no_browser=False,
                )
        self.assertTrue(callback_done.wait(2))
        self.assertEqual(callback_response["status"], 400)
        self.assertIn("身份验证未完成", str(callback_response["body"]))
        self.assertNotIn("wrong-state", str(callback_response["body"]))
        self.assertNotIn("secret-code", str(callback_response["body"]))

    def test_login_uses_code_flow_mints_oact_and_never_caches_refresh_token(self) -> None:
        session = self.manager.login(self.settings)
        self.assertEqual(session.company_id, "org-example")
        self.assertEqual(session.oact, "short-oact-1")
        self.assertEqual(self.http.forms[0]["grant_type"], "authorization_code")
        self.assertNotIn("password", self.http.forms[0])
        self.assertTrue(self.http.forms[0]["code_verifier"])
        self.assertEqual(
            self.http.mints[0]["value"],
            {
                "requested_scopes": list(DEFAULT_CAPABILITY_SCOPES),
            },
        )
        self.assertEqual(
            self.http.mints[0]["url"],
            "https://semattice.example.test/v1/auth/token",
        )
        self.assertEqual(self.http.mints[0]["bearer"], "keycloak-access-1")
        self.assertIn("keycloak-refresh-1", self.store.values.values())
        raw_cache = self.cache_path.read_text(encoding="utf-8")
        self.assertNotIn("keycloak-refresh", raw_cache)
        self.assertEqual(stat.S_IMODE(self.cache_path.stat().st_mode), 0o600)
        self.assertEqual(stat.S_IMODE(self.cache_path.parent.stat().st_mode), 0o700)

    def test_expired_oact_refreshes_keycloak_and_rotates_refresh_token(self) -> None:
        first = self.manager.login(self.settings)
        self.now_value[0] = first.oact_expires_at
        renewed = self.manager.get_session()
        self.assertEqual(renewed.oact, "short-oact-2")
        self.assertEqual(self.http.forms[-1]["grant_type"], "refresh_token")
        self.assertEqual(self.http.forms[-1]["refresh_token"], "keycloak-refresh-1")
        self.assertIn("keycloak-refresh-2", self.store.values.values())
        self.assertEqual(self.http.mints[-1]["bearer"], "keycloak-access-2")

    def test_logout_removes_secure_and_short_lived_credentials(self) -> None:
        session = self.manager.login(self.settings)
        self.manager.logout(self.settings)
        self.assertFalse(self.cache_path.exists())
        self.assertNotIn(session.credential_account, self.store.values)

    def test_cache_rejects_wide_permissions_and_symbolic_links(self) -> None:
        cache = SessionCache(self.cache_path)
        session = CachedSession(
            version=SESSION_CACHE_VERSION,
            issuer=self.settings.issuer,
            client_id=self.settings.client_id,
            semattice_base_url=self.settings.semattice_base_url,
            company_id="org-example",
            scopes=[],
            credential_account="account",
            oact="short-oact",
            oact_expires_at=2_000,
        )
        cache.save(session)
        os.chmod(self.cache_path, 0o644)
        with self.assertRaisesRegex(AuthError, "权限过宽"):
            cache.load()

        self.cache_path.unlink()
        target = Path(self.temp_dir.name) / "target.json"
        target.write_text("{}", encoding="utf-8")
        self.cache_path.symlink_to(target)
        with self.assertRaisesRegex(AuthError, "符号链接"):
            cache.load()

    def test_windows_cache_uses_local_appdata_without_posix_mode_checks(self) -> None:
        local_app_data = Path(self.temp_dir.name) / "LocalAppData"
        with mock.patch("semattice_auth.platform.system", return_value="Windows"):
            with mock.patch.dict(
                os.environ,
                {
                    "LOCALAPPDATA": str(local_app_data),
                    "SEMATTICE_CREDENTIALS_FILE": "",
                },
                clear=False,
            ):
                self.assertEqual(
                    default_credentials_file(),
                    local_app_data / "CloudCC" / "Semattice" / "credentials.json",
                )
            session = CachedSession(
                version=SESSION_CACHE_VERSION,
                issuer=self.settings.issuer,
                client_id=self.settings.client_id,
                semattice_base_url=self.settings.semattice_base_url,
                company_id="org-example",
                scopes=list(DEFAULT_CAPABILITY_SCOPES),
                credential_account="account",
                oact="short-oact",
                oact_expires_at=2_000,
            )
            cache = SessionCache(self.cache_path)
            cache.save(session)
            os.chmod(self.cache_path, 0o644)
            self.assertEqual(cache.load(), session)

    def test_windows_credential_manager_saves_loads_and_deletes_utf8_secret(self) -> None:
        saved: dict[str, bytes] = {}
        allocations: list[object] = []
        last_error = [0]

        def write(credential_reference: object, _flags: int) -> bool:
            credential = credential_reference._obj
            saved[credential.TargetName] = ctypes.string_at(
                credential.CredentialBlob, credential.CredentialBlobSize
            )
            return True

        def read(target: str, _kind: int, _flags: int, output: object) -> bool:
            secret = saved.get(target)
            if secret is None:
                last_error[0] = WindowsCredentialStore._ERROR_NOT_FOUND
                return False
            blob = (ctypes.c_ubyte * len(secret)).from_buffer_copy(secret)
            credential = _WindowsCredential()
            credential.CredentialBlobSize = len(secret)
            credential.CredentialBlob = ctypes.cast(blob, ctypes.POINTER(ctypes.c_ubyte))
            credential_pointer = ctypes.pointer(credential)
            ctypes.cast(
                output,
                ctypes.POINTER(ctypes.POINTER(_WindowsCredential)),
            )[0] = credential_pointer
            allocations.extend([blob, credential, credential_pointer])
            return True

        def delete(target: str, _kind: int, _flags: int) -> bool:
            if target in saved:
                del saved[target]
                return True
            last_error[0] = WindowsCredentialStore._ERROR_NOT_FOUND
            return False

        store = WindowsCredentialStore.__new__(WindowsCredentialStore)
        store.advapi32 = SimpleNamespace(
            CredWriteW=write,
            CredReadW=read,
            CredDeleteW=delete,
            CredFree=lambda _value: None,
        )
        store._last_error = lambda: last_error[0]

        store.save("account", "刷新令牌-refresh-token")
        self.assertEqual(store.load("account"), "刷新令牌-refresh-token")
        store.delete("account")
        store.delete("account")
        with self.assertRaisesRegex(AuthError, "重新登录"):
            store.load("account")

    def test_system_credential_store_routes_windows_to_credential_manager(self) -> None:
        sentinel = object()
        with mock.patch("semattice_auth.platform.system", return_value="Windows"):
            with mock.patch("semattice_auth.WindowsCredentialStore", return_value=sentinel):
                self.assertIs(system_credential_store(), sentinel)

    def test_cache_rejects_pre_all_capability_scope_version(self) -> None:
        self.cache_path.parent.mkdir(mode=0o700, parents=True)
        self.cache_path.write_text(
            json.dumps(
                {
                    "version": 2,
                    "issuer": self.settings.issuer,
                    "client_id": self.settings.client_id,
                    "semattice_base_url": self.settings.semattice_base_url,
                    "company_id": "org-example",
                    "scopes": ["system.capability.read"],
                    "credential_account": "account",
                    "oact": "old-oact",
                    "oact_expires_at": 2_000,
                }
            ),
            encoding="utf-8",
        )
        os.chmod(self.cache_path, 0o600)
        with self.assertRaisesRegex(AuthError, "版本不兼容"):
            SessionCache(self.cache_path).load()

    def test_401_retries_once_with_refreshed_oact_and_same_body(self) -> None:
        calls: list[tuple[str, bytes | None]] = []

        def opener(request: object, *, timeout: float) -> FakeResponse:
            del timeout
            calls.append((request.headers["Authorization"], request.data))
            if len(calls) == 1:
                raise urllib.error.HTTPError(
                    request.full_url,
                    401,
                    "Unauthorized",
                    {},
                    io.BytesIO(b'{"status":"failed"}'),
                )
            return FakeResponse(200, {"status": "succeeded", "result": {}})

        body = {"request_id": "req-stable", "idempotency_key": "idem-stable", "input": {}}
        status_code, payload = semattice_api.invoke_with_auth_retry(
            "https://semattice.example.test/v1/capabilities/test/invoke",
            body,
            "old-oact",
            10,
            refresh=lambda: "new-oact",
            urlopen=opener,
        )
        self.assertEqual(status_code, 200)
        self.assertEqual(payload["status"], "succeeded")
        self.assertEqual([item[0] for item in calls], ["Bearer old-oact", "Bearer new-oact"])
        self.assertEqual(calls[0][1], calls[1][1])

    def test_unreadable_http_error_body_returns_stable_failure(self) -> None:
        def opener(request: object, *, timeout: float) -> FakeResponse:
            del timeout
            raise urllib.error.HTTPError(
                request.full_url,
                302,
                "Found",
                {},
                ResettingErrorBody(),
            )

        status_code, payload = semattice_api.invoke_once(
            "https://semattice.example.test/v1/capabilities/test/invoke",
            {"request_id": "req-reset", "input": {}},
            "short-oact",
            10,
            urlopen=opener,
        )
        self.assertEqual(status_code, 302)
        self.assertEqual(payload["status"], "failed")
        self.assertEqual(payload["http_status"], 302)

    def test_explicit_token_takes_priority_over_login_cache(self) -> None:
        args = Namespace(
            input="{}",
            input_file=None,
            request_id="req-explicit",
            idempotency_key=None,
            capability="system.capability.list",
            token_env="TEST_SEMATTICE_TOKEN",
            base_url="https://semattice.example.test",
            credentials_file=self.cache_path,
            timeout=30.0,
            dry_run=False,
        )
        with mock.patch.dict(os.environ, {"TEST_SEMATTICE_TOKEN": "explicit-oact"}, clear=False):
            with mock.patch.object(
                semattice_api,
                "invoke_with_auth_retry",
                return_value=(200, {"status": "succeeded"}),
            ) as invoke:
                self.assertEqual(semattice_api.run_call(args), 0)
        self.assertEqual(invoke.call_args.args[2], "explicit-oact")
        self.assertIsNone(invoke.call_args.kwargs["refresh"])


if __name__ == "__main__":
    unittest.main()
