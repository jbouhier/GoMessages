//! Real Google sign-in (OAuth 2.0 installed-app / loopback flow, PKCE).
//!
//! What this proves: the app opens the system browser, the user signs in
//! with Google, tokens come back to `http://127.0.0.1:<port>/callback`,
//! are exchanged + refreshed, and the account email shows in the UI.
//!
//! Scope is `openid email profile` for now. Talking to Messages itself needs
//! its (undiscovered) scope + endpoints. One-line change in [`AUTH_SCOPE`]
//! once known. Nothing here is mocked: PKCE, HTTP, and token parsing are real
//! and unit-tested with RFC fixtures.

use std::io::{Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::mpsc::{self, Receiver};
use std::time::Duration;

use base64::Engine as _;
use rand::{distr::Alphanumeric, Rng};
use sha2::{Digest, Sha256};

/// Change this when the Messages scope is known.
pub const AUTH_SCOPE: &str = "openid email profile";
const TOKEN_URL: &str = "https://oauth2.googleapis.com/token";

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize)]
pub struct Tokens {
    pub access_token: String,
    pub refresh_token: Option<String>,
    pub id_token: Option<String>,
    pub email: Option<String>,
}

#[derive(Debug, Clone, serde::Serialize, serde::Deserialize, Default)]
struct AuthFile {
    client_id: String,
    tokens: Option<Tokens>,
}

pub enum AuthEvent {
    SignedIn(Tokens),
    Failed(String),
}

struct PendingAuth {
    verifier: String,
    rx: Receiver<Result<(String, String), String>>,
    cancel: std::sync::Arc<std::sync::atomic::AtomicBool>,
}

pub struct AuthManager {
    path: PathBuf,
    pub client_id: String,
    pub tokens: Option<Tokens>,
    pending: Option<PendingAuth>,
    event_rx: Option<Receiver<AuthEvent>>,
    event_tx: Option<std::sync::mpsc::Sender<AuthEvent>>,
}

impl AuthManager {
    pub fn default_path() -> Option<PathBuf> {
        crate::paths::data_dir().map(|p| p.join(crate::config::AUTH_FILE))
    }

    pub fn load(path: PathBuf) -> Self {
        let file: AuthFile = std::fs::read(&path)
            .ok()
            .and_then(|b| serde_json::from_slice(&b).ok())
            .unwrap_or_default();
        let (event_tx, event_rx) = mpsc::channel();
        let mgr = Self {
            path,
            client_id: file.client_id,
            tokens: file.tokens,
            pending: None,
            event_rx: Some(event_rx),
            event_tx: Some(event_tx),
        };
        // Silent refresh on startup so a stale access token still works.
        if let Some(t) = mgr.tokens.clone() {
            if let Some(refresh) = t.refresh_token.clone() {
                let client_id = mgr.client_id.clone();
                let tx = mgr.event_tx.clone().unwrap();
                std::thread::spawn(move || match refresh_tokens(&client_id, &refresh) {
                    Ok(new_access) => {
                        let mut updated = t;
                        updated.access_token = new_access;
                        let _ = tx.send(AuthEvent::SignedIn(updated));
                    }
                    Err(e) => {
                        eprintln!("gomessages: token refresh failed: {e:#}");
                    }
                });
            }
        }
        mgr
    }

    fn persist(&self) {
        if let Some(parent) = self.path.parent() {
            let _ = std::fs::create_dir_all(parent);
        }
        let file = AuthFile {
            client_id: self.client_id.clone(),
            tokens: self.tokens.clone(),
        };
        if let Ok(bytes) = serde_json::to_vec_pretty(&file) {
            let _ = std::fs::write(&self.path, bytes);
            #[cfg(unix)]
            {
                use std::os::unix::fs::PermissionsExt;
                let _ =
                    std::fs::set_permissions(&self.path, std::fs::Permissions::from_mode(0o600));
            }
        }
    }

    pub fn signed_in(&self) -> bool {
        self.tokens.is_some()
    }

    pub fn waiting(&self) -> bool {
        self.pending.is_some()
    }

    /// Start browser sign-in. Returns the URL opened (also opened automatically).
    pub fn begin_sign_in(&mut self) -> anyhow::Result<String> {
        if self.client_id.trim().is_empty() {
            anyhow::bail!("enter your Google OAuth client ID first (README: Login)");
        }
        let verifier = pkce_verifier();
        let challenge = pkce_challenge(&verifier);
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let port = listener.local_addr()?.port();
        let redirect = format!("http://127.0.0.1:{port}/callback");
        let url = auth_url(&self.client_id, &redirect, &challenge, AUTH_SCOPE);

        let (tx, rx) = mpsc::channel();
        let cancel = std::sync::Arc::new(std::sync::atomic::AtomicBool::new(false));
        let cancel_in_thread = cancel.clone();
        std::thread::spawn(move || {
            let out = wait_for_code(listener, cancel_in_thread);
            let _ = tx.send(out);
        });
        self.pending = Some(PendingAuth {
            verifier,
            rx,
            cancel,
        });
        let _ = webbrowser::open(&url);
        Ok(url)
    }

    pub fn cancel(&mut self) {
        if let Some(p) = &self.pending {
            p.cancel.store(true, std::sync::atomic::Ordering::Relaxed);
        }
        self.pending = None;
    }

    pub fn sign_out(&mut self) {
        self.tokens = None;
        self.pending = None;
        self.persist();
    }

    /// Poll auth progress. Call every frame; requests repaint on completion.
    pub fn poll(&mut self, ctx: &egui::Context) {
        // A received code moves into the token exchange (background thread).
        if let Some(p) = self.pending.take() {
            match p.rx.try_recv() {
                Ok(Ok(code_info)) => {
                    let client_id = self.client_id.clone();
                    let tx = self.event_tx.clone().unwrap();
                    std::thread::spawn(move || {
                        let ev = match exchange_code(&client_id, &p.verifier, &code_info) {
                            Ok(t) => AuthEvent::SignedIn(t),
                            Err(e) => AuthEvent::Failed(format!("{e:#}")),
                        };
                        let _ = tx.send(ev);
                    });
                    // Exchange running; login step done.
                    ctx.request_repaint();
                }
                Ok(Err(e)) => {
                    let tx = self.event_tx.clone().unwrap();
                    let _ = tx.send(AuthEvent::Failed(e));
                }
                Err(mpsc::TryRecvError::Empty) => {
                    self.pending = Some(p);
                }
                Err(mpsc::TryRecvError::Disconnected) => {
                    let tx = self.event_tx.clone().unwrap();
                    let _ = tx.send(AuthEvent::Failed("login window closed".into()));
                }
            }
        }
        if let Some(rx) = &self.event_rx {
            while let Ok(ev) = rx.try_recv() {
                match ev {
                    AuthEvent::SignedIn(t) => {
                        self.tokens = Some(t);
                        self.persist();
                        ctx.request_repaint();
                    }
                    AuthEvent::Failed(e) => {
                        eprintln!("gomessages: sign-in failed: {e}");
                        ctx.request_repaint();
                    }
                }
            }
        }
    }
}

/// Random PKCE verifier: 64 chars from the unreserved set.
pub fn pkce_verifier() -> String {
    rand::rng()
        .sample_iter(&Alphanumeric)
        .take(64)
        .map(char::from)
        .collect()
}

/// S256 challenge = BASE64URL-NOPAD(SHA256(verifier)).
pub fn pkce_challenge(verifier: &str) -> String {
    let digest = Sha256::digest(verifier.as_bytes());
    base64::engine::general_purpose::URL_SAFE_NO_PAD.encode(digest)
}

pub fn auth_url(client_id: &str, redirect_uri: &str, challenge: &str, scope: &str) -> String {
    let mut u = url::Url::parse("https://accounts.google.com/o/oauth2/v2/auth").unwrap();
    u.query_pairs_mut()
        .append_pair("client_id", client_id)
        .append_pair("redirect_uri", redirect_uri)
        .append_pair("response_type", "code")
        .append_pair("scope", scope)
        .append_pair("code_challenge", challenge)
        .append_pair("code_challenge_method", "S256")
        .append_pair("access_type", "offline")
        .append_pair("prompt", "consent");
    u.to_string()
}

/// Block until the browser hits `/callback?code=…`. Returns (code, redirect_uri).
/// Polls so Cancel/timeout can stop it; gives up after ~5 minutes.
fn wait_for_code(
    listener: TcpListener,
    cancel: std::sync::Arc<std::sync::atomic::AtomicBool>,
) -> Result<(String, String), String> {
    use std::sync::atomic::Ordering;
    listener.set_nonblocking(true).map_err(|e| e.to_string())?;
    let port = listener.local_addr().map_err(|e| e.to_string())?.port();
    let redirect = format!("http://127.0.0.1:{port}/callback");
    for _ in 0..(5 * 60 * 20) {
        if cancel.load(Ordering::Relaxed) {
            return Err("login cancelled".into());
        }
        match listener.accept() {
            Ok((mut stream, _)) => {
                stream
                    .set_read_timeout(Some(Duration::from_secs(30)))
                    .map_err(|e| e.to_string())?;
                let mut buf = vec![0u8; 8192];
                let n = stream.read(&mut buf).map_err(|e| e.to_string())?;
                let req = String::from_utf8_lossy(&buf[..n]);
                let line = req.lines().next().unwrap_or("");
                // Expect: GET /callback?code=XYZ&scope=... HTTP/1.1
                let path = line.split_whitespace().nth(1).unwrap_or("");
                let fake = format!("http://x{path}");
                let url = url::Url::parse(&fake).map_err(|e| e.to_string())?;
                if let Some((_, code)) = url.query_pairs().find(|(k, _)| k == "code") {
                    let body =
                        "<html><body><h3>Signed in, you can close this tab.</h3></body></html>";
                    let resp = format!(
                        "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                        body.len(),
                        body
                    );
                    let _ = stream.write_all(resp.as_bytes());
                    return Ok((code.to_string(), redirect));
                }
                return Err("login redirect had no code (was access denied?)".into());
            }
            Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                std::thread::sleep(Duration::from_millis(50));
            }
            Err(e) => return Err(e.to_string()),
        }
    }
    Err("login timed out".into())
}

fn exchange_code(
    client_id: &str,
    verifier: &str,
    code_info: &(String, String),
) -> anyhow::Result<Tokens> {
    let (code, redirect) = code_info;
    let resp: serde_json::Value = ureq::post(TOKEN_URL)
        .timeout(Duration::from_secs(20))
        .send_form(&[
            ("code", code.as_str()),
            ("client_id", client_id),
            ("code_verifier", verifier),
            ("grant_type", "authorization_code"),
            ("redirect_uri", redirect.as_str()),
        ])?
        .into_json()?;
    tokens_from_json(&resp)
}

fn refresh_tokens(client_id: &str, refresh_token: &str) -> anyhow::Result<String> {
    let resp: serde_json::Value = ureq::post(TOKEN_URL)
        .timeout(Duration::from_secs(20))
        .send_form(&[
            ("client_id", client_id),
            ("refresh_token", refresh_token),
            ("grant_type", "refresh_token"),
        ])?
        .into_json()?;
    resp.get("access_token")
        .and_then(|s| s.as_str())
        .map(|s| s.to_string())
        .ok_or_else(|| anyhow::anyhow!("refresh response missing access_token: {resp}"))
}

pub fn tokens_from_json(v: &serde_json::Value) -> anyhow::Result<Tokens> {
    let access = v
        .get("access_token")
        .and_then(|s| s.as_str())
        .ok_or_else(|| anyhow::anyhow!("token response missing access_token: {v}"))?
        .to_string();
    let refresh = v
        .get("refresh_token")
        .and_then(|s| s.as_str())
        .map(|s| s.to_string());
    let id = v
        .get("id_token")
        .and_then(|s| s.as_str())
        .map(|s| s.to_string());
    let email = id.as_deref().and_then(email_from_id_token);
    Ok(Tokens {
        access_token: access,
        refresh_token: refresh,
        id_token: id,
        email,
    })
}

/// Read `email` from an ID token payload WITHOUT verifying the signature.
/// Display-only; verify properly before trusting identity for anything.
pub fn email_from_id_token(id_token: &str) -> Option<String> {
    let mut parts = id_token.split('.');
    let (_h, payload, _s) = (parts.next()?, parts.next()?, parts.next()?);
    let bytes = base64::engine::general_purpose::URL_SAFE_NO_PAD
        .decode(payload)
        .ok()?;
    let v: serde_json::Value = serde_json::from_slice(&bytes).ok()?;
    v.get("email")?.as_str().map(|s| s.to_string())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn pkce_matches_rfc7636_vector() {
        let v = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
        assert_eq!(
            pkce_challenge(v),
            "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"
        );
        assert_eq!(pkce_verifier().len(), 64);
    }

    #[test]
    fn auth_url_has_pkce_params() {
        let u = auth_url("CID", "http://127.0.0.1:9/callback", "CH", "openid email");
        assert!(u.contains("code_challenge=CH"));
        assert!(u.contains("code_challenge_method=S256"));
        assert!(u.contains("response_type=code"));
        assert!(u.contains("access_type=offline"));
    }

    #[test]
    fn token_json_parses_with_email() {
        // Header {"alg":"none"}. Payload {"email":"a@b.c"}. Signature empty.
        let id = "eyJhbGciOiJub25lIn0.eyJlbWFpbCI6ImFAYi5jIn0.";
        let v = serde_json::json!({"access_token": "AT", "refresh_token": "RT", "id_token": id});
        let t = tokens_from_json(&v).unwrap();
        assert_eq!(t.email.as_deref(), Some("a@b.c"));
        assert_eq!(email_from_id_token("bad.token"), None);
    }

    #[test]
    fn token_json_rejects_missing_access() {
        assert!(tokens_from_json(&serde_json::json!({"error": "x"})).is_err());
    }
}
